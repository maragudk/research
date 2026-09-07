package main

import (
	"context"
	"fmt"
	"time"

	"filippo.io/age"
	"filippo.io/age/plugin"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

// PluginName is the age plugin name: recipients are "age1atproto1...", and the
// binary is "age-plugin-atproto".
const PluginName = "atproto"

// EncodeRecipient produces the "age1atproto1..." string for an identifier. The
// Bech32 payload is simply the normalized handle or DID as UTF-8 bytes.
func EncodeRecipient(identifier string) (string, error) {
	atid, err := NormalizeIdentifier(identifier)
	if err != nil {
		return "", err
	}
	return plugin.EncodeRecipient(PluginName, []byte(atid.String())), nil
}

// pluginRecipient is the age.Recipient the plugin exposes to age. Wrapping a
// file key triggers the network lookup; the resulting stanzas are native
// X25519 (or hybrid) stanzas, so the receiver decrypts with plain age.
type pluginRecipient struct {
	p        *plugin.Plugin
	resolver *Resolver
	atid     syntax.AtIdentifier
	cached   *Result
}

var _ age.RecipientWithLabels = (*pluginRecipient)(nil)

func newPluginRecipient(p *plugin.Plugin, resolver *Resolver, data []byte) (*pluginRecipient, error) {
	atid, err := NormalizeIdentifier(string(data))
	if err != nil {
		return nil, err
	}
	return &pluginRecipient{p: p, resolver: resolver, atid: atid}, nil
}

func (r *pluginRecipient) lookup() (*Result, error) {
	if r.cached != nil {
		return r.cached, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := r.resolver.Lookup(ctx, r.atid.String())
	if err != nil {
		return nil, err
	}
	r.cached = res
	// Tell the user what was resolved. This is the only place they can notice
	// a surprising DID behind a familiar-looking handle.
	_ = r.p.DisplayMessage(describe(res))
	return res, nil
}

func describe(res *Result) string {
	handle := "no verified handle"
	if res.Handle != syntax.HandleInvalid {
		handle = "@" + res.Handle.String()
	}
	return fmt.Sprintf("encrypting to %s (%s): %d recipient(s) from %s at rev %s",
		handle, res.DID, len(res.Recipients), res.URI(), res.Rev)
}

func (r *pluginRecipient) Wrap(fileKey []byte) ([]*age.Stanza, error) {
	stanzas, _, err := r.WrapWithLabels(fileKey)
	return stanzas, err
}

// WrapWithLabels wraps the file key to every published recipient. The stanza
// set is labeled "postquantum" only if every recipient is post-quantum; a mix
// is wrapped without the label, because the file is only as strong as its
// weakest stanza.
func (r *pluginRecipient) WrapWithLabels(fileKey []byte) ([]*age.Stanza, []string, error) {
	res, err := r.lookup()
	if err != nil {
		return nil, nil, err
	}
	var stanzas []*age.Stanza
	allPQ := true
	for _, rec := range res.Recipients {
		s, err := rec.Recipient.Wrap(fileKey)
		if err != nil {
			return nil, nil, fmt.Errorf("wrapping to %s: %w", truncate(rec.Encoded), err)
		}
		stanzas = append(stanzas, s...)
		allPQ = allPQ && rec.PostQuantum
	}
	var labels []string
	if allPQ {
		labels = []string{"postquantum"}
	}
	return stanzas, labels, nil
}
