package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEndToEndWithAgeBinary drives the real age CLI: it encrypts to an
// age1atproto1... recipient through this plugin (built into a temp dir on
// PATH) and decrypts with the fixture's plain age identity, proving the
// receiving side needs no plugin.
func TestEndToEndWithAgeBinary(t *testing.T) {
	ageBin, err := exec.LookPath("age")
	if err != nil {
		t.Skip("age binary not on PATH")
	}
	f := newFixture(t)

	dir := t.TempDir()
	build := exec.Command("go", "build", "-o", filepath.Join(dir, "age-plugin-atproto"), ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building plugin: %v\n%s", err, out)
	}

	recipient, err := EncodeRecipient(fixtureDID)
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		envPLCURL+"="+f.server.URL,
		envAllowPrivate+"=1",
	)

	encrypted := filepath.Join(dir, "message.txt.age")
	enc := exec.Command(ageBin, "-r", recipient, "-o", encrypted)
	enc.Env = env
	enc.Stdin = strings.NewReader("Hello!\n")
	var stderr bytes.Buffer
	enc.Stderr = &stderr
	if err := enc.Run(); err != nil {
		t.Fatalf("age encrypt: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "encrypting to") || !strings.Contains(stderr.String(), fixtureDID) {
		t.Errorf("expected the plugin to report what it resolved, stderr was:\n%s", stderr.String())
	}

	keyFile := filepath.Join(dir, "key.txt")
	if err := os.WriteFile(keyFile, []byte(f.identity.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dec := exec.Command(ageBin, "-d", "-i", keyFile, encrypted)
	dec.Env = os.Environ() // no plugin, no network overrides: plain age
	out, err := dec.CombinedOutput()
	if err != nil {
		t.Fatalf("age decrypt: %v\n%s", err, out)
	}
	if string(out) != "Hello!\n" {
		t.Fatalf("decrypted %q", out)
	}

	// The header must contain a native X25519 stanza and nothing plugin-specific.
	raw, _ := os.ReadFile(encrypted)
	if !bytes.Contains(raw, []byte("-> X25519 ")) {
		t.Errorf("no native X25519 stanza in header:\n%s", raw[:min(len(raw), 200)])
	}
	if bytes.Contains(raw, []byte("atproto")) {
		t.Errorf("header leaks plugin name:\n%s", raw[:min(len(raw), 200)])
	}
}
