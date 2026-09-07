package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// cmdPublish writes the recipients record to the caller's own repository. It
// uses an app password (com.atproto.server.createSession) because that is the
// simplest thing that works from a CLI today; an OAuth flow would be the
// production answer.
func cmdPublish(args []string) int {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	identifier := fs.String("identifier", "", "your handle or DID")
	password := fs.String("app-password", os.Getenv("AGE_PLUGIN_ATPROTO_APP_PASSWORD"), "app password (or set AGE_PLUGIN_ATPROTO_APP_PASSWORD)")
	pdsOverride := fs.String("pds", "", "PDS URL (default: from your DID document)")
	labels := fs.String("labels", "", "comma-separated labels, one per recipient, in order")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: age-plugin-atproto publish -identifier <@handle|did> [-app-password PW] [-labels a,b] <age1...> [<age1...>...]")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *identifier == "" || fs.NArg() == 0 {
		fs.Usage()
		return 2
	}
	if *password == "" {
		fatalf("an app password is required (-app-password or AGE_PLUGIN_ATPROTO_APP_PASSWORD)")
	}

	var labelList []string
	if *labels != "" {
		labelList = strings.Split(*labels, ",")
		if len(labelList) != fs.NArg() {
			fatalf("-labels has %d entries but %d recipients were given", len(labelList), fs.NArg())
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var entries []map[string]any
	for i, s := range fs.Args() {
		rec, err := ParseNativeRecipient(s)
		if err != nil {
			fatalf("recipient %d: %v", i+1, err)
		}
		e := map[string]any{"recipient": rec.Encoded, "createdAt": now}
		if labelList != nil && labelList[i] != "" {
			e["label"] = labelList[i]
		}
		entries = append(entries, e)
	}

	ctx := context.Background()
	resolver := NewResolver()
	atid, err := NormalizeIdentifier(*identifier)
	if err != nil {
		fatalf("%v", err)
	}
	pds := *pdsOverride
	did := ""
	if pds == "" {
		ident, err := resolver.Dir.Lookup(ctx, atid)
		if err != nil {
			fatalf("resolving %s: %v", atid, err)
		}
		pds = ident.PDSEndpoint()
		did = ident.DID.String()
		if pds == "" {
			fatalf("DID document for %s declares no PDS", ident.DID)
		}
	}
	if err := checkPDSURL(pds); err != nil {
		fatalf("%v", err)
	}
	pds = strings.TrimSuffix(pds, "/")

	// Log in.
	var session struct {
		AccessJwt string `json:"accessJwt"`
		DID       string `json:"did"`
	}
	if err := xrpcPost(ctx, resolver.HTTP, pds, "com.atproto.server.createSession", "",
		map[string]any{"identifier": atid.String(), "password": *password}, &session); err != nil {
		fatalf("login failed: %v", err)
	}
	if did != "" && session.DID != did {
		fatalf("logged in as %s but %s resolved to %s", session.DID, atid, did)
	}

	// Write the record. rkey "self" means this replaces any previous record.
	record := map[string]any{
		"$type":      DefaultCollection,
		"recipients": entries,
	}
	var out struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := xrpcPost(ctx, resolver.HTTP, pds, "com.atproto.repo.putRecord", session.AccessJwt, map[string]any{
		"repo":       session.DID,
		"collection": DefaultCollection,
		"rkey":       DefaultRKey,
		"record":     record,
	}, &out); err != nil {
		fatalf("publishing record: %v", err)
	}
	fmt.Printf("published %s (cid %s) with %d recipient(s)\n", out.URI, out.CID, len(entries))
	return 0
}

func xrpcPost(ctx context.Context, client *http.Client, pds, nsid, bearer string, input any, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pds+"/xrpc/"+nsid, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "age-plugin-atproto/0.1")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		var xe struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(respBody, &xe)
		if xe.Error != "" {
			return fmt.Errorf("%s: %s (HTTP %d)", xe.Error, xe.Message, resp.StatusCode)
		}
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, nsid)
	}
	if output != nil {
		return json.Unmarshal(respBody, output)
	}
	return nil
}
