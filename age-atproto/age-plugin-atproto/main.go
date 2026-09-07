// Command age-plugin-atproto is an age plugin that encrypts to atproto accounts.
//
// As a plugin (invoked by age):
//
//	age -r age1atproto1... file > file.age
//
// As a tool:
//
//	age-plugin-atproto recipient @markus.maragu.dev   # print the age1atproto1... recipient
//	age-plugin-atproto lookup @markus.maragu.dev      # print the verified native recipients
//	age-plugin-atproto publish -identifier @me age1... # publish recipients to your repo
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"filippo.io/age"
	"filippo.io/age/plugin"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "recipient":
			return cmdRecipient(args[1:])
		case "lookup":
			return cmdLookup(args[1:])
		case "publish":
			return cmdPublish(args[1:])
		case "help", "-h", "--help":
			usage()
			return 0
		}
	}

	p, err := plugin.New(PluginName)
	if err != nil {
		fatalf("%v", err)
	}
	fs := flag.NewFlagSet("age-plugin-atproto", flag.ExitOnError)
	fs.Usage = usage
	p.RegisterFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	resolver := NewResolver()
	p.HandleRecipient(func(data []byte) (age.Recipient, error) {
		return newPluginRecipient(p, resolver, data)
	})
	return p.Main()
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage:
  age-plugin-atproto recipient <@handle|did>          Print the age1atproto1... recipient for age -r
  age-plugin-atproto lookup <@handle|did>             Resolve, verify and print the published native recipients
  age-plugin-atproto publish -identifier <@handle|did> [-app-password PW] <age1...> [<age1...>...]
                                                      Publish (replace) your recipients record
  age-plugin-atproto --age-plugin=recipient-v1        Plugin protocol (invoked by age)

Environment:
  AGE_PLUGIN_ATPROTO_APP_PASSWORD           App password for publish (instead of -app-password)
  AGE_PLUGIN_ATPROTO_PLC_URL                Override the PLC directory URL (default https://plc.directory)
  AGE_PLUGIN_ATPROTO_ALLOW_PRIVATE_NETWORK  Set to 1 to allow http:// and private addresses (tests only)
`)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "age-plugin-atproto: "+format+"\n", args...)
	os.Exit(1)
}

func cmdRecipient(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: age-plugin-atproto recipient <@handle|did>")
		return 2
	}
	r, err := EncodeRecipient(args[0])
	if err != nil {
		fatalf("%v", err)
	}
	fmt.Println(r)
	return 0
}

func cmdLookup(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: age-plugin-atproto lookup <@handle|did>")
		return 2
	}
	res, err := NewResolver().Lookup(context.Background(), args[0])
	if err != nil {
		fatalf("%v", err)
	}
	// Output is a valid age recipients file (age -R): comments, then one
	// recipient per line.
	handle := "handle.invalid"
	if res.Handle != syntax.HandleInvalid {
		handle = res.Handle.String()
	}
	fmt.Printf("# %s\n# handle: %s\n# record: %s (cid %s)\n# verified against commit rev %s signed by %s#atproto, served by %s\n",
		res.DID, handle, res.URI(), res.RecordCID, res.Rev, res.DID, res.PDS)
	for _, rec := range res.Recipients {
		if rec.Label != "" {
			fmt.Printf("# %s\n", strings.ReplaceAll(rec.Label, "\n", " "))
		}
		fmt.Println(rec.Encoded)
	}
	return 0
}
