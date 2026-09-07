package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/bluesky-social/indigo/atproto/atdata"
	"github.com/bluesky-social/indigo/atproto/identity"
	atrepo "github.com/bluesky-social/indigo/atproto/repo"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/util/ssrf"
)

// DefaultCollection is the NSID of the record type holding an account's age recipients.
const DefaultCollection = "dev.maragu.age.recipients"

// DefaultRKey is the record key: exactly one record per account.
const DefaultRKey = "self"

// maxCARBytes bounds how much proof data we accept from a PDS.
const maxCARBytes = 4 << 20

// Environment variables understood by the resolver. They exist for tests and
// for people running their own PLC directory; production use needs none of them.
const (
	envPLCURL       = "AGE_PLUGIN_ATPROTO_PLC_URL"
	envAllowPrivate = "AGE_PLUGIN_ATPROTO_ALLOW_PRIVATE_NETWORK"
)

// Recipient is one age recipient published by an account.
type Recipient struct {
	// Encoded is the recipient string exactly as published, e.g. "age1...".
	Encoded string
	// Label is an optional human label from the record ("laptop", "yubikey backup").
	Label string
	// Recipient is the parsed native age recipient.
	Recipient age.Recipient
	// PostQuantum is true for ML-KEM-768+X25519 hybrid recipients.
	PostQuantum bool
}

// Result is a verified lookup.
type Result struct {
	// Identifier is the normalized identifier the user asked for (handle or DID).
	Identifier string
	DID        syntax.DID
	// Handle is the bidirectionally verified handle, or "handle.invalid".
	Handle syntax.Handle
	PDS    string
	// Rev is the repository revision (a TID) of the signed commit the record was proven against.
	Rev        string
	RecordCID  string
	Recipients []Recipient
}

// URI returns the AT URI of the record that was verified.
func (r *Result) URI() string {
	return fmt.Sprintf("at://%s/%s/%s", r.DID, DefaultCollection, DefaultRKey)
}

// Resolver turns an atproto identifier into a verified set of age recipients.
type Resolver struct {
	Dir        identity.Directory
	HTTP       *http.Client
	Collection syntax.NSID
	RKey       syntax.RecordKey
}

// NewResolver builds a resolver with production defaults (SSRF-protected HTTP,
// public PLC directory), honoring the test/override environment variables.
func NewResolver() *Resolver {
	allowPrivate := os.Getenv(envAllowPrivate) == "1"
	plcURL := os.Getenv(envPLCURL)
	if plcURL == "" {
		plcURL = identity.DefaultPLCURL
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if !allowPrivate {
		transport.DialContext = ssrf.PublicOnlyDialer().DialContext
	}
	httpClient := http.Client{Timeout: 15 * time.Second, Transport: transport}

	base := &identity.BaseDirectory{
		PLCURL:     plcURL,
		HTTPClient: httpClient,
		PLCClient:  &http.Client{Timeout: 15 * time.Second, Transport: transport},
		Resolver: net.Resolver{
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 3 * time.Second}
				return d.DialContext(ctx, network, address)
			},
		},
		TryAuthoritativeDNS:   true,
		SkipDNSDomainSuffixes: []string{".bsky.social"},
		UserAgent:             "age-plugin-atproto/0.1",
	}

	return &Resolver{
		Dir:        base,
		HTTP:       &httpClient,
		Collection: syntax.NSID(DefaultCollection),
		RKey:       syntax.RecordKey(DefaultRKey),
	}
}

// NormalizeIdentifier accepts "@handle", "handle", or "did:..." and returns a
// parsed, normalized AT identifier.
func NormalizeIdentifier(s string) (syntax.AtIdentifier, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	atid, err := syntax.ParseAtIdentifier(s)
	if err != nil {
		return "", fmt.Errorf("%q is not an atproto handle or DID: %w", s, err)
	}
	return atid.Normalize(), nil
}

// Lookup resolves identifier to its account, fetches the recipients record
// together with a proof from the account's PDS, and verifies the proof against
// the signing key in the DID document. Nothing returned here is taken on the
// PDS's word alone.
func (r *Resolver) Lookup(ctx context.Context, identifier string) (*Result, error) {
	atid, err := NormalizeIdentifier(identifier)
	if err != nil {
		return nil, err
	}

	// Step 1: identity. For a handle this does handle -> DID resolution (DNS TXT
	// or HTTPS well-known), then DID -> document, then checks that the document
	// claims the handle back (bidirectional verification). For a DID it resolves
	// the document and reports the handle as invalid if it doesn't round-trip.
	ident, err := r.Dir.Lookup(ctx, atid)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", atid, err)
	}
	if atid.IsHandle() && ident.Handle != atid.Handle() {
		// Defensive: BaseDirectory.LookupHandle already errors on mismatch.
		return nil, fmt.Errorf("handle %s does not belong to %s", atid, ident.DID)
	}

	// Step 2: the two things we need from the DID document.
	signingKey, err := ident.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("DID document for %s has no usable #atproto signing key: %w", ident.DID, err)
	}
	pds := ident.PDSEndpoint()
	if pds == "" {
		return nil, fmt.Errorf("DID document for %s declares no PDS", ident.DID)
	}
	if err := checkPDSURL(pds); err != nil {
		return nil, err
	}

	// Step 3: fetch the record with its Merkle proof (signed commit + MST path).
	car, err := r.fetchRecordProof(ctx, pds, ident.DID)
	if err != nil {
		return nil, err
	}

	// Step 4: verify. go-car checks every block's hash against its CID while
	// reading, so the MST path and the record bytes are bound to the commit's
	// "data" root. The commit signature binds that root to the account's key.
	commit, repo, err := atrepo.LoadRepoFromCAR(ctx, bytes.NewReader(car))
	if err != nil {
		return nil, fmt.Errorf("parsing proof from %s: %w", pds, err)
	}
	if commit.DID != ident.DID.String() {
		return nil, fmt.Errorf("proof commit is for %s, expected %s", commit.DID, ident.DID)
	}
	if err := commit.VerifySignature(signingKey); err != nil {
		return nil, fmt.Errorf("commit signature from %s does not verify against %s#atproto: %w", pds, ident.DID, err)
	}
	if err := checkRev(commit.Rev); err != nil {
		return nil, err
	}
	recordBytes, recordCID, err := repo.GetRecordBytes(ctx, r.Collection, r.RKey)
	if errors.Is(err, atrepo.ErrNotFound) {
		return nil, fmt.Errorf("%s has not published any age recipients (no %s record)", atid, r.Collection)
	}
	if err != nil {
		return nil, fmt.Errorf("locating record in proof: %w", err)
	}

	// Step 5: parse the record.
	recipients, err := parseRecipientsRecord(recordBytes, r.Collection)
	if err != nil {
		return nil, fmt.Errorf("record at://%s/%s/%s: %w", ident.DID, r.Collection, r.RKey, err)
	}

	return &Result{
		Identifier: atid.String(),
		DID:        ident.DID,
		Handle:     ident.Handle,
		PDS:        pds,
		Rev:        commit.Rev,
		RecordCID:  recordCID.String(),
		Recipients: recipients,
	}, nil
}

func checkPDSURL(pds string) error {
	u, err := url.Parse(pds)
	if err != nil {
		return fmt.Errorf("invalid PDS URL %q: %w", pds, err)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && os.Getenv(envAllowPrivate) == "1" {
		return nil
	}
	return fmt.Errorf("refusing to contact PDS over %s: %q", u.Scheme, pds)
}

// checkRev rejects commits claiming to be from the future. This is a weak
// freshness check; see the README on rollback by a malicious PDS.
func checkRev(rev string) error {
	tid, err := syntax.ParseTID(rev)
	if err != nil {
		return fmt.Errorf("commit rev %q is not a TID: %w", rev, err)
	}
	if tid.Time().After(time.Now().Add(10 * time.Minute)) {
		return fmt.Errorf("commit rev %s is in the future", rev)
	}
	return nil
}

func (r *Resolver) fetchRecordProof(ctx context.Context, pds string, did syntax.DID) ([]byte, error) {
	q := url.Values{}
	q.Set("did", did.String())
	q.Set("collection", r.Collection.String())
	q.Set("rkey", r.RKey.String())
	endpoint := strings.TrimSuffix(pds, "/") + "/xrpc/com.atproto.sync.getRecord?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.ipld.car")
	req.Header.Set("User-Agent", "age-plugin-atproto/0.1")
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching record proof from %s: %w", pds, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCARBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading record proof from %s: %w", pds, err)
	}
	if len(body) > maxCARBytes {
		return nil, fmt.Errorf("record proof from %s exceeds %d bytes", pds, maxCARBytes)
	}
	if resp.StatusCode != http.StatusOK {
		var xe struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &xe)
		if xe.Error == "RecordNotFound" {
			return nil, fmt.Errorf("%s has not published any age recipients (no %s record)", did, r.Collection)
		}
		if xe.Error != "" {
			return nil, fmt.Errorf("PDS %s returned %s: %s", pds, xe.Error, xe.Message)
		}
		return nil, fmt.Errorf("PDS %s returned HTTP %d", pds, resp.StatusCode)
	}
	return body, nil
}

// parseRecipientsRecord decodes a dev.maragu.age.recipients record and parses
// each recipient. Only native age recipients are accepted: a record fetched
// from the network must never be able to choose which plugin binary runs.
func parseRecipientsRecord(b []byte, collection syntax.NSID) ([]Recipient, error) {
	obj, err := atdata.UnmarshalCBOR(b)
	if err != nil {
		return nil, fmt.Errorf("decoding record: %w", err)
	}
	if typ, _ := obj["$type"].(string); typ != collection.String() {
		return nil, fmt.Errorf("record $type is %q, expected %q", typ, collection)
	}
	items, ok := obj["recipients"].([]any)
	if !ok {
		return nil, errors.New("record has no recipients array")
	}
	if len(items) == 0 {
		return nil, errors.New("record lists no recipients")
	}
	if len(items) > 32 {
		return nil, fmt.Errorf("record lists %d recipients, maximum is 32", len(items))
	}
	var out []Recipient
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("recipients[%d] is not an object", i)
		}
		enc, _ := m["recipient"].(string)
		label, _ := m["label"].(string)
		rec, err := ParseNativeRecipient(enc)
		if err != nil {
			return nil, fmt.Errorf("recipients[%d]: %w", i, err)
		}
		rec.Label = label
		out = append(out, rec)
	}
	return out, nil
}

// ParseNativeRecipient parses an age recipient string, accepting only the
// native X25519 ("age1...") and hybrid post-quantum ("age1pq1...") encodings.
func ParseNativeRecipient(s string) (Recipient, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "age1pq1"):
		r, err := age.ParseHybridRecipient(s)
		if err != nil {
			return Recipient{}, fmt.Errorf("invalid hybrid recipient: %w", err)
		}
		return Recipient{Encoded: s, Recipient: r, PostQuantum: true}, nil
	case strings.HasPrefix(s, "age1") && strings.Count(s, "1") > 1:
		return Recipient{}, fmt.Errorf("plugin recipient %q not allowed: only native age recipients may be published", truncate(s))
	case strings.HasPrefix(s, "age1"):
		r, err := age.ParseX25519Recipient(s)
		if err != nil {
			return Recipient{}, fmt.Errorf("invalid X25519 recipient: %w", err)
		}
		return Recipient{Encoded: s, Recipient: r}, nil
	case strings.HasPrefix(s, "ssh-"):
		return Recipient{}, errors.New("SSH recipients are not supported; publish an age1... recipient")
	default:
		return Recipient{}, fmt.Errorf("unknown recipient type %q", truncate(s))
	}
}

func truncate(s string) string {
	if len(s) > 24 {
		return s[:24] + "..."
	}
	return s
}
