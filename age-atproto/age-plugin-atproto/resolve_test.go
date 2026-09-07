package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/bluesky-social/indigo/atproto/atcrypto"
)

func TestLookupVerifiesAndReturnsRecipients(t *testing.T) {
	f := newFixture(t)
	res, err := NewResolver().Lookup(context.Background(), fixtureDID)
	if err != nil {
		t.Fatal(err)
	}
	if res.DID.String() != fixtureDID {
		t.Fatalf("DID = %s", res.DID)
	}
	if len(res.Recipients) != 1 {
		t.Fatalf("got %d recipients", len(res.Recipients))
	}
	if got, want := res.Recipients[0].Encoded, f.identity.Recipient().String(); got != want {
		t.Fatalf("recipient = %s, want %s", got, want)
	}
	if res.Recipients[0].Label != "test laptop" {
		t.Fatalf("label = %q", res.Recipients[0].Label)
	}
	if res.Recipients[0].PostQuantum {
		t.Fatal("X25519 recipient flagged post-quantum")
	}

	// The returned recipient must actually work with plain age.
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, res.Recipients[0].Recipient)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("Hello!"))
	_ = w.Close()
	r, err := age.Decrypt(&buf, f.identity)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
	if out.String() != "Hello!" {
		t.Fatalf("decrypted %q", out.String())
	}
}

func TestLookupAcceptsAtPrefixedDID(t *testing.T) {
	newFixture(t)
	if _, err := NewResolver().Lookup(context.Background(), "@"+fixtureDID); err != nil {
		t.Fatal(err)
	}
}

func TestLookupRejectsCommitSignedByWrongKey(t *testing.T) {
	f := newFixture(t)
	other, err := atcrypto.GeneratePrivateKeyK256()
	if err != nil {
		t.Fatal(err)
	}
	f.commitSigner = other
	_, err = NewResolver().Lookup(context.Background(), fixtureDID)
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("expected signature error, got %v", err)
	}
}

func TestLookupRejectsDIDDocumentWithDifferentKey(t *testing.T) {
	f := newFixture(t)
	other, err := atcrypto.GeneratePrivateKeyK256()
	if err != nil {
		t.Fatal(err)
	}
	f.docSigningKey, _ = other.PublicKey()
	_, err = NewResolver().Lookup(context.Background(), fixtureDID)
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("expected signature error, got %v", err)
	}
}

func TestLookupRejectsTamperedRecordBlock(t *testing.T) {
	f := newFixture(t)
	good := f.buildCAR()
	// Flip a byte inside the record's recipient string. go-car detects the
	// CID/content mismatch, so the tampered block never reaches the parser.
	needle := []byte(f.identity.Recipient().String())
	i := bytes.Index(good, needle)
	if i < 0 {
		t.Fatal("recipient not found in CAR")
	}
	bad := bytes.Clone(good)
	bad[i+10] ^= 0x01
	f.carOverride = bad
	_, err := NewResolver().Lookup(context.Background(), fixtureDID)
	if err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("expected content integrity error, got %v", err)
	}
}

func TestLookupRejectsProofForAnotherAccount(t *testing.T) {
	f := newFixture(t)
	f.did = "did:plc:bbbbbbbbbbbbbbbbbbbbbbbb"
	f.carOverride = f.buildCAR() // signed by the right key, but commit.did differs
	f.did = fixtureDID
	_, err := NewResolver().Lookup(context.Background(), fixtureDID)
	if err == nil || !strings.Contains(err.Error(), "proof commit is for") {
		t.Fatalf("expected DID mismatch error, got %v", err)
	}
}

func TestLookupReportsMissingRecord(t *testing.T) {
	f := newFixture(t)
	f.record = nil
	_, err := NewResolver().Lookup(context.Background(), fixtureDID)
	if err == nil || !strings.Contains(err.Error(), "has not published") {
		t.Fatalf("expected not-published error, got %v", err)
	}
}

func TestLookupRejectsPluginRecipientsInRecord(t *testing.T) {
	f := newFixture(t)
	f.record["recipients"] = []any{map[string]any{"recipient": "age1yubikey1qwt50d05nh5vutpdzmlg5wn80xq5negm8gq7c5pvuk5yq7kae8x4gqt0uyy"}}
	_, err := NewResolver().Lookup(context.Background(), fixtureDID)
	if err == nil || !strings.Contains(err.Error(), "plugin recipient") {
		t.Fatalf("expected plugin recipient rejection, got %v", err)
	}
}

func TestLookupRejectsWrongRecordType(t *testing.T) {
	f := newFixture(t)
	f.record["$type"] = "app.bsky.actor.profile"
	_, err := NewResolver().Lookup(context.Background(), fixtureDID)
	if err == nil || !strings.Contains(err.Error(), "$type") {
		t.Fatalf("expected $type error, got %v", err)
	}
}

func TestParseNativeRecipient(t *testing.T) {
	id, _ := age.GenerateX25519Identity()
	if _, err := ParseNativeRecipient(id.Recipient().String()); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"ssh-ed25519 AAAA", "age1tag1abc", "AGE-SECRET-KEY-1", "", "hello"} {
		if _, err := ParseNativeRecipient(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestEncodeRecipient(t *testing.T) {
	r, err := EncodeRecipient("@Markus.Maragu.dev")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r, "age1atproto1") {
		t.Fatalf("recipient = %s", r)
	}
	r2, _ := EncodeRecipient("markus.maragu.dev")
	if r != r2 {
		t.Fatalf("normalization differs: %s vs %s", r, r2)
	}
	if _, err := EncodeRecipient("not a handle"); err == nil {
		t.Fatal("accepted invalid identifier")
	}
}

func TestPublishWritesRecord(t *testing.T) {
	f := newFixture(t)
	id, _ := age.GenerateX25519Identity()
	code := cmdPublish([]string{"-identifier", fixtureDID, "-app-password", "app-pass", "-labels", "phone", id.Recipient().String()})
	if code != 0 {
		t.Fatalf("publish exit code %d", code)
	}
	if len(f.putRecords) != 1 {
		t.Fatalf("%d putRecord calls", len(f.putRecords))
	}
	in := f.putRecords[0]
	if in["collection"] != DefaultCollection || in["rkey"] != DefaultRKey || in["repo"] != fixtureDID {
		t.Fatalf("unexpected putRecord input: %v", in)
	}
	// And what was published must round-trip through a verified lookup.
	res, err := NewResolver().Lookup(context.Background(), fixtureDID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Recipients[0].Encoded != id.Recipient().String() || res.Recipients[0].Label != "phone" {
		t.Fatalf("round-trip mismatch: %+v", res.Recipients[0])
	}
}
