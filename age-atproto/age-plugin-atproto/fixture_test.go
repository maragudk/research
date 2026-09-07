package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/atdata"
	atrepo "github.com/bluesky-social/indigo/atproto/repo"
	"github.com/bluesky-social/indigo/atproto/repo/mst"
	"github.com/bluesky-social/indigo/atproto/syntax"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipld/go-car"
	carutil "github.com/ipld/go-car/util"
	"github.com/multiformats/go-multihash"
)

// fixture is a fake atproto account: a signing key, a DID document served by a
// fake PLC directory, and a fake PDS that serves a signed proof for the
// recipients record. It is enough to exercise every verification step offline.
type fixture struct {
	t          *testing.T
	did        syntax.DID
	signingKey *atcrypto.PrivateKeyK256
	identity   *age.X25519Identity
	server     *httptest.Server

	// mutable knobs for negative tests
	record        map[string]any
	docSigningKey atcrypto.PublicKey // key advertised in the DID document
	commitSigner  atcrypto.PrivateKey
	carOverride   []byte
	putRecords    []map[string]any
}

const fixtureDID = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"

func newFixture(t *testing.T) *fixture {
	t.Helper()
	priv, err := atcrypto.GeneratePrivateKeyK256()
	if err != nil {
		t.Fatal(err)
	}
	pub, err := priv.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{
		t:             t,
		did:           syntax.DID(fixtureDID),
		signingKey:    priv,
		identity:      id,
		docSigningKey: pub,
		commitSigner:  priv,
		record: map[string]any{
			"$type": DefaultCollection,
			"recipients": []any{
				map[string]any{"recipient": id.Recipient().String(), "label": "test laptop", "createdAt": "2026-09-07T00:00:00Z"},
			},
		},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	t.Setenv(envPLCURL, f.server.URL)
	t.Setenv(envAllowPrivate, "1")
	return f
}

func (f *fixture) handle(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/"+f.did.String():
		// Fake PLC directory: DID document.
		doc := map[string]any{
			"@context":    []string{"https://www.w3.org/ns/did/v1", "https://w3id.org/security/multikey/v1"},
			"id":          f.did.String(),
			"alsoKnownAs": []string{},
			"verificationMethod": []map[string]any{{
				"id":                 f.did.String() + "#atproto",
				"type":               "Multikey",
				"controller":         f.did.String(),
				"publicKeyMultibase": f.docSigningKey.Multibase(),
			}},
			"service": []map[string]any{{
				"id":              "#atproto_pds",
				"type":            "AtprotoPersonalDataServer",
				"serviceEndpoint": f.server.URL,
			}},
		}
		w.Header().Set("Content-Type", "application/did+ld+json")
		_ = json.NewEncoder(w).Encode(doc)
	case r.URL.Path == "/xrpc/com.atproto.sync.getRecord":
		if r.URL.Query().Get("did") != f.did.String() {
			http.Error(w, `{"error":"RepoNotFound"}`, 404)
			return
		}
		if f.record == nil {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"RecordNotFound","message":"Could not locate record"}`))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.ipld.car")
		if f.carOverride != nil {
			_, _ = w.Write(f.carOverride)
			return
		}
		_, _ = w.Write(f.buildCAR())
	case r.URL.Path == "/xrpc/com.atproto.server.createSession":
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in["password"] != "app-pass" {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":"AuthenticationRequired","message":"Invalid identifier or password"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"accessJwt": "token", "refreshJwt": "r", "did": f.did.String(), "handle": "handle.invalid"})
	case r.URL.Path == "/xrpc/com.atproto.repo.putRecord":
		if r.Header.Get("Authorization") != "Bearer token" {
			w.WriteHeader(401)
			return
		}
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		f.putRecords = append(f.putRecords, in)
		f.record = in["record"].(map[string]any)
		_ = json.NewEncoder(w).Encode(map[string]any{"uri": fmt.Sprintf("at://%s/%s/%s", f.did, in["collection"], in["rkey"]), "cid": "bafyfake"})
	default:
		http.NotFound(w, r)
	}
}

// recordingBlockstore keeps the blocks written by WriteDiffBlocks with their
// exact CIDs. (AllKeysChan on the map blockstore would return raw-codec CIDs.)
type recordingBlockstore struct {
	blockstore.Blockstore
	blocks []blocks.Block
}

func (r *recordingBlockstore) Put(ctx context.Context, b blocks.Block) error {
	r.blocks = append(r.blocks, b)
	return r.Blockstore.Put(ctx, b)
}

func (r *recordingBlockstore) PutMany(ctx context.Context, bs []blocks.Block) error {
	r.blocks = append(r.blocks, bs...)
	return r.Blockstore.PutMany(ctx, bs)
}

var dagCBORPrefix = cid.Prefix{Version: 1, Codec: cid.DagCBOR, MhType: multihash.SHA2_256, MhLength: -1}

// buildCAR produces what com.atproto.sync.getRecord returns: the signed commit,
// the MST nodes on the path to the record, and the record block.
func (f *fixture) buildCAR() []byte {
	t := f.t
	ctx := context.Background()

	recordBytes, err := atdata.MarshalCBOR(f.record)
	if err != nil {
		t.Fatal(err)
	}
	recordCID, err := dagCBORPrefix.Sum(recordBytes)
	if err != nil {
		t.Fatal(err)
	}

	// A few unrelated records so the MST has some shape.
	tree := mst.NewEmptyTree()
	otherCID, _ := dagCBORPrefix.Sum([]byte("other"))
	for _, k := range []string{"app.bsky.feed.post/3l2aaaaaaaaaa", "app.bsky.feed.post/3l2bbbbbbbbbb", "app.bsky.actor.profile/self"} {
		if _, err := tree.Insert([]byte(k), otherCID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tree.Insert([]byte(DefaultCollection+"/"+DefaultRKey), recordCID); err != nil {
		t.Fatal(err)
	}
	bs := &recordingBlockstore{Blockstore: blockstore.NewBlockstore(datastore.NewMapDatastore())}
	root, err := tree.WriteDiffBlocks(ctx, bs)
	if err != nil {
		t.Fatal(err)
	}

	commit := atrepo.Commit{
		DID:     f.did.String(),
		Version: 3,
		Prev:    nil,
		Data:    *root,
		Rev:     syntax.NewTIDFromTime(time.Now().Add(-time.Minute), 0).String(),
	}
	if err := commit.Sign(f.commitSigner); err != nil {
		t.Fatal(err)
	}
	var commitBuf bytes.Buffer
	if err := commit.MarshalCBOR(&commitBuf); err != nil {
		t.Fatal(err)
	}
	commitCID, err := dagCBORPrefix.Sum(commitBuf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := car.WriteHeader(&car.CarHeader{Roots: []cid.Cid{commitCID}, Version: 1}, &out); err != nil {
		t.Fatal(err)
	}
	if err := carutil.LdWrite(&out, commitCID.Bytes(), commitBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	for _, blk := range bs.blocks {
		if err := carutil.LdWrite(&out, blk.Cid().Bytes(), blk.RawData()); err != nil {
			t.Fatal(err)
		}
	}
	if err := carutil.LdWrite(&out, recordCID.Bytes(), recordBytes); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
