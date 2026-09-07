# Encrypting with age to an ATproto recipient

Research into `echo Hello! | age -r @markus.maragu.dev > message.txt.age`: publishing an [age](https://age-encryption.org) public key on the ATproto network under its own lexicon, looking it up at encryption time through the [age plugin system](https://c2sp.org/age-plugin), and making sure the key really belongs to the identity.

A working prototype lives in [`age-plugin-atproto/`](age-plugin-atproto/) and the lexicon in [`lexicons/`](lexicons/). Everything below was verified against the specs and source code linked in [Sources](#sources), and the prototype is tested offline against a mock PLC directory and PDS, including an end-to-end run through the stock `age` binary. It has **not** been run against the live network yet, because this environment cannot reach `plc.directory` or any PDS.

## TL;DR

- **It works, with one UX compromise.** Stock `age` will never accept `-r @markus.maragu.dev`: plugin recipients must be Bech32 strings of the form `age1<plugin>1...`, and Filippo already tried and removed a network-lookup recipient (`github:user`). The stock-compatible spelling is `age -r age1atproto1d4shy6m4wvhx6ctjv9nh2tnyv4mqe09mzg`, which is just the handle in Bech32. A two-line shell function gets you the exact command you asked for.
- **Publish the key as a repo record, not in the DID document.** A `dev.maragu.age.recipients` record with rkey `self` is writable by any account holder with an app password or OAuth session, holds several keys (laptop, phone, YubiKey backup, post-quantum), and is verifiable: `com.atproto.sync.getRecord` returns the record together with the signed repo commit and the Merkle path to it. The DID document alternative is possible since June 2025 (PLC now accepts any `did:key` in `verificationMethods`) but requires a PLC operation and gives you at most 10 keys with 32-character ids; it is worth keeping as a later "stronger binding" option, see [Where to publish](#where-to-publish-the-key).
- **The plugin only does work on the encrypt side.** It resolves handle → DID → DID document → PDS, fetches the record proof, verifies the commit signature against the account's `#atproto` key, then wraps the file key with age's *native* X25519 (or hybrid post-quantum) stanzas. The recipient decrypts with plain `age -d -i key.txt`; no plugin, no network, and the file header does not even mention atproto.
- **The trust you get is "whoever controls the account's signing key".** For a Bluesky-hosted account that is the PDS operator, so Bluesky could substitute a key. Self-hosting a PDS, or holding your own PLC rotation key and putting the key in the DID document, moves that trust to you. That is the honest ceiling of this design, and it is the same ceiling the account's posts already live under.

## Target UX and what age allows

The command from the request cannot work verbatim. `cmd/age/parse.go` dispatches recipients by prefix: `age1tag1`/`age1tagpq1`, `age1pq1`, `age1<name>1...` (plugin), `age1...` (X25519), `ssh-...`, and a stub for `github:` that only returns the error `"github:" recipients were removed from the design`. Anything else is `unknown recipient type`. The plugin spec is explicit about why: the recipient string alone has to identify the plugin binary to run, without configuration, and the binary is looked up on `PATH` as `age-plugin-<name>`. Filippo's plugins post adds the security argument, that the encoding "prevents untrusted files from controlling which plugins execute". An upstream `-r @handle` or `-r atproto:handle` would be a new recipient syntax hardwired to one network, which is exactly what `github:` was and what got removed. I would not expect a PR for it to land.

What does work with any age ≥ 1.1 (the prototype was tested with the age 1.1.1 installed here):

```console
$ age-plugin-atproto recipient @markus.maragu.dev
age1atproto1d4shy6m4wvhx6ctjv9nh2tnyv4mqe09mzg

$ echo Hello! | age -r age1atproto1d4shy6m4wvhx6ctjv9nh2tnyv4mqe09mzg > message.txt.age
age-plugin-atproto: encrypting to @markus.maragu.dev (did:plc:...): 2 recipient(s) from at://did:plc:.../dev.maragu.age.recipients/self at rev 3lxyz...
```

The Bech32 payload is nothing more than the normalized handle (or DID) as bytes, so the recipient string is stable, copy-pasteable, and can be put in a `-R` recipients file. Two other spellings that also need no upstream change:

```sh
# Resolve up front and encrypt with plain age (no plugin at encrypt time either):
age -R <(age-plugin-atproto lookup @markus.maragu.dev) file > file.age

# The command from the request, as a shell function:
age() { local a=(); for x in "$@"; do case "$x" in @*) x=$(age-plugin-atproto recipient "$x") ;; esac; a+=("$x"); done; command age "${a[@]}"; }
echo Hello! | age -r @markus.maragu.dev > message.txt.age
```

`lookup` prints a valid recipients file: comment lines with the DID, verified handle, record URI and CID, and commit revision, then one native recipient per line. This is the form to use in scripts, because it separates "resolve and verify" (network, fallible) from "encrypt" (offline).

## How the plugin side works

The plugin protocol is a stanza-based stdin/stdout state machine. For encryption age runs `age-plugin-atproto --age-plugin=recipient-v1`, sends `add-recipient <bech32>` and `wrap-file-key <key>`, then expects `recipient-stanza` responses. The Go framework in `filippo.io/age/plugin` hides all of that: you register a function turning the decoded payload into an `age.Recipient`, and the framework runs the protocol.

Three properties of the protocol matter for this design:

1. **Stanzas are not tied to recipient types.** A plugin may emit any stanza, including the native `X25519` and `mlkem768x25519` ones. The prototype's recipient resolves the account, parses the published `age1...` strings with `age.ParseX25519Recipient` / `age.ParseHybridRecipient`, and calls their `Wrap`. The resulting header is indistinguishable from `age -r age1...`. The end-to-end test asserts that the header contains an `X25519` stanza and never the string `atproto`, and that plain `age -d -i key.txt` decrypts it.
2. **The plugin can talk to the user.** `Plugin.DisplayMessage` sends a `msg` stanza that age prints to the terminal. The prototype uses it to state which handle, DID, record, and revision it resolved. That line is the only chance the encrypting party has to notice a lookalike handle or a surprising DID, so it is not optional.
3. **Labels.** Since age 1.3 a recipient can attach labels; `postquantum` stanzas can only be combined with other `postquantum` stanzas. A published set mixing a classic `age1...` key and a hybrid `age1pq1...` key therefore has to be wrapped *without* the label (the file is only as strong as its weakest stanza). The prototype labels the set `postquantum` only if every published recipient is hybrid.

There is no identity side: the account holder's secret key is an ordinary age identity. The plugin never sees it, and `age-plugin-atproto` is not needed on the decrypting machine.

## Where to publish the key

ATproto gives an account two authenticated places to put public data. Both were investigated; the prototype uses the first.

| | Repo record (`dev.maragu.age.recipients`) | DID document `verificationMethod` |
|---|---|---|
| Who can write it | The account, with an app password or OAuth session (`com.atproto.repo.putRecord`) | A PLC rotation key holder. Bluesky-hosted users can go through `identity.requestPlcOperationSignature` (email code) + `signPlcOperation` + `submitPlcOperation`; did:web users edit the JSON file |
| What signs it | The `#atproto` repo signing key, via the commit signature | The PLC rotation keys, via the operation log (or TLS+DNS for did:web) |
| Who holds that key for a typical Bluesky user | The PDS (Bluesky) | Also the PDS, unless the user added their own rotation key ahead of Bluesky's |
| Verification path | `sync.getRecord` proof: commit signature + Merkle path, all offline once fetched | Trust plc.directory, or replay the audit log (`/:did/log/audit`) yourself |
| Capacity | 32 recipients in the prototype lexicon, arbitrary metadata | Max 10 verification methods total, ids ≤ 32 chars, keys ≤ 256 chars, whole operation ≤ 4000 bytes (server `constraints.ts`). A hybrid PQ key does not fit in 256 chars |
| Key format | Any string; we store age's own Bech32 encodings | Must be `did:key:` + base58btc multikey. X25519 has multicodec `0xec`, so `did:key:z6LS...` would be legal since the June 2025 relaxation; hybrid ML-KEM has no registered multicodec |
| Rotation/removal | Just put a new record | New PLC op, subject to the 72-hour recovery window in which a higher-priority rotation key can rewrite it |
| Precedent | Tangled's `sh.tangled.publicKey` (SSH keys) and atmo.sh use exactly this pattern | `#atproto_label` for labeler signing keys |

The record wins on writability, capacity, and format. The DID document wins on one thing: for a user who holds their own rotation key, it binds the age key to the *identity* rather than to the PDS-held signing key. That is a real difference, but it only applies to users who already opted into holding rotation keys, and it cannot express a hybrid PQ key at all. A future version of the plugin could look for `#age` in the DID document first and fall back to the record; the lookup code already parses all verification methods, so this is a small addition.

Bluesky's own position (discussion #121) is that encrypted *content* should not go in public repos because it "raises the stakes for key loss and content leaking". A public *key* record has none of that problem; it is the same kind of thing as a profile or an SSH key record.

### Lexicon

[`lexicons/dev.maragu.age.recipients.json`](lexicons/dev.maragu.age.recipients.json). Design choices:

- **Record key `literal:self`, one record holding an array**, instead of one record per key with TID keys the way Tangled does. Reason: `sync.getRecord` proves one record at a time, and `listRecords` has no proof, so a per-key layout lets a malicious PDS silently omit keys. With one record the proof covers the whole set. The cost is read-modify-write on the client when adding a key, which `putRecord`'s `swapRecord` handles.
- **Native age recipients only.** `age1...` and `age1pq1...` are accepted; `age1yubikey1...`, `age1tag1...`, and SSH keys are rejected at parse time. A plugin recipient in the record would mean data fetched from the network decides which binary runs on the encrypting machine, the exact thing the plugin naming rules exist to prevent. Hardware-key users publish the *native* recipient their hardware plugin gives them, or a software key. SSH keys are excluded because age's own SSH support is a compatibility feature with weaker security properties (discussion #540) and Tangled already covers "SSH keys on atproto" if someone wants a bridge.
- **`label` and `createdAt` are informational.** Nothing verifies them. There is no `expiresAt`; removing a key from the record is the revocation mechanism, and encrypting parties see the current record.
- **NSID.** `dev.maragu.age.recipients` puts the authority at `age.maragu.dev`, so publishing the schema as a `com.atproto.lexicon.schema` record needs a `_lexicon.age.maragu.dev` TXT record. If this became a community thing it should move to a neutral namespace; the plugin has the collection name in one constant.

### Publishing

```console
$ age-keygen -o key.txt
Public key: age1yhm4gctwfmrpz87tdslm550wrx6m79y9f2hdzt0lndjnehwj0ukqrjpyx5
$ AGE_PLUGIN_ATPROTO_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx \
  age-plugin-atproto publish -identifier @markus.maragu.dev -labels laptop age1yhm4gctwfmrpz87tdslm550wrx6m79y9f2hdzt0lndjnehwj0ukqrjpyx5
published at://did:plc:.../dev.maragu.age.recipients/self (cid bafy...) with 1 recipient(s)
```

`publish` logs in with `com.atproto.server.createSession` and writes the record with `putRecord`. It replaces the whole list, so pass every key each time. App passwords are the simplest CLI-friendly auth today; OAuth is the right answer for anything user-facing. Any other atproto tool (goat, pdsls, a curl to `putRecord`) can write the same record.

## Lookup and verification, step by step

This is what `Resolver.Lookup` in [`resolve.go`](age-plugin-atproto/resolve.go) does, and what each step buys.

1. **Normalize the identifier.** Strip `@`, lowercase handles (they are case-insensitive), accept DIDs as-is.
2. **Handle → DID.** DNS TXT at `_atproto.<handle>` (`did=did:...`), or HTTPS `GET https://<handle>/.well-known/atproto-did`. Neither channel is authenticated beyond DNS and TLS. If both answer with different DIDs, DNS wins. Done by indigo's `identity.BaseDirectory`.
3. **DID → DID document.** `https://plc.directory/<did>` for did:plc, `https://<host>/.well-known/did.json` for did:web. Only those two methods are blessed in atproto.
4. **Bidirectional check.** The DID document's `alsoKnownAs` must list `at://<handle>`; otherwise the lookup fails. Without this anyone could point a handle they control at somebody else's DID and make their keys look like that person's. When you look up by DID instead, the handle is reported as `handle.invalid` if it does not round-trip, and the plugin says so.
5. **Extract the signing key and PDS.** `verificationMethod` entry with id `#atproto`, type `Multikey`, `publicKeyMultibase` = base58btc(multicodec ‖ compressed point), where the multicodec is `0xE7 0x01` for k256 or `0x80 0x24` for p256. `service` entry `#atproto_pds` gives the PDS URL, which must be https.
6. **Fetch the proof.** `GET <pds>/xrpc/com.atproto.sync.getRecord?did=…&collection=dev.maragu.age.recipients&rkey=self` returns a CAR v1 file whose root is the signed commit, plus the MST nodes on the path to the record and the record block. Size is capped at 4 MiB. A 400 `RecordNotFound` means the account has not published keys.
7. **Verify block integrity.** go-car recomputes every block's hash and rejects mismatches with the CAR's CIDs, so nothing in the file can be altered without breaking a link (tested: flipping one byte of the record fails with a content-integrity error).
8. **Verify the commit.** `commit.did` must equal the DID we resolved (a PDS could otherwise hand us a valid proof from a different account it hosts). Then ECDSA over SHA-256 of the DAG-CBOR encoding of the commit without its `sig` field, low-S enforced, against the `#atproto` key from step 5. This is the step that makes the PDS untrusted for content: the key comes from the DID document, the proof comes from the PDS, and they must agree.
9. **Walk the MST** from `commit.data` to `dev.maragu.age.recipients/self`. Missing nodes off the path are fine (partial tree); a missing node *on* the path is an error.
10. **Parse the record.** DAG-CBOR → map, `$type` must equal the collection, every `recipients[].recipient` must parse as a native age recipient.

Negative tests cover a commit signed by another key, a DID document advertising a different key, a tampered block, a proof for a different DID, a missing record, a wrong `$type`, and a plugin recipient string inside the record.

## Nuances and threat model

**The signing key is usually held by the PDS.** For Bluesky-hosted accounts the PDS has both the `#atproto` signing key and the highest-priority rotation key. The proof therefore says "the current holder of this account's signing key published this", not "the human did". A Bluesky-scale operator substituting an age key on a target account is detectable (the firehose would carry the commit, and the victim's own `lookup` would show a key they do not own) but not preventable. Users who care can self-host their PDS, and the plugin needs no change for that. Users who hold their own rotation key could additionally publish the key in the DID document; the operation log then proves *they* signed it. The general lesson: this is exactly as strong as the account's posts, which for most users is what they want.

**Handles are convenient, DIDs are the anchor.** A handle can be transferred: markus.maragu.dev today could be a different account after a domain sale, and DNS answers are unauthenticated unless DNSSEC is used, which atproto does not require. The plugin always prints the resolved DID, `lookup` writes it into the recipients file comments, and `age1atproto1...` can encode a DID instead of a handle. For anything long-lived (scripts, CI), pin the DID: `age-plugin-atproto recipient did:plc:...`. Trust-on-first-use pinning (remember DID and key set per handle, warn on change, the way ssh does) is an obvious follow-up and was left out of the prototype to keep it stateless.

**Rollback and omission by the PDS.** The proof shows the record existed in *a* signed commit; nothing proves it is the *latest* commit. A malicious or compromised PDS could keep serving a proof for an old revision containing a key the user removed. Mitigations, none implemented yet: compare `commit.rev` with what a relay reports (`com.atproto.sync.getLatestCommit` on `bsky.network`, which observes the firehose), or remember the last seen rev per DID and refuse to go backwards. Rev is at least checked not to be in the future. Omission (claiming `RecordNotFound`) is a denial of service, not a confidentiality problem, and the plugin fails closed.

**PLC directory trust.** `plc.directory` is a single operator (Bluesky). Its attacks are limited to withholding or serving the wrong fork of an operation history, both within the 72-hour recovery window rules. Fully verifying the audit log (replay every operation, check rotation-key signatures and nullification) is possible offline and indigo does not do it by default; neither does the prototype. It is the same trust every atproto client places today.

**Key rotation.** Rotating the repo signing key requires a new commit; the spec says the latest commit must always verify with the current DID document. So an old proof fails after a rotation, which is fine for a live lookup. Rotating the age key is just publishing a new record; files already encrypted are unaffected, which is the normal age property.

**Privacy.** Encrypting to a handle leaks to your DNS resolver, plc.directory, and the recipient's PDS that you are encrypting to them right now. The file header itself leaks nothing (native stanzas, no handle). If that matters, resolve once with `lookup` and reuse the recipients file.

**Network safety in the plugin.** The PDS URL comes from a DID document controlled by whoever registered the DID, so the plugin makes HTTP requests to attacker-chosen hosts. The prototype uses indigo's SSRF-protected dialer (public addresses only), requires https, caps the response size, and applies timeouts. The `AGE_PLUGIN_ATPROTO_ALLOW_PRIVATE_NETWORK=1` escape hatch exists only for the tests.

**age-specific edge cases.** Encrypting to a handle plus other recipients works normally (stanzas just accumulate). Passphrase (`-p`) cannot be combined with any recipient, as usual. A record with only hybrid PQ recipients yields `postquantum`-labeled stanzas, which age 1.3 refuses to mix with classic recipients on the same command line, matching how `age1pq1` behaves natively. Very long handles are fine for Bech32; age's decoder does not enforce the 90-character limit.

## Prototype

```
age-atproto/
├── README.md                          this document
├── lexicons/dev.maragu.age.recipients.json
└── age-plugin-atproto/
    ├── main.go        CLI: plugin mode, recipient, lookup, publish
    ├── resolve.go     identity resolution, proof fetching and verification, record parsing
    ├── recipient.go   the age.Recipient exposed through the plugin protocol
    ├── publish.go     createSession + putRecord
    ├── fixture_test.go  fake PLC directory + PDS serving a real signed proof
    ├── resolve_test.go  positive and negative verification tests
    └── e2e_test.go      stock age binary → plugin → file → plain age decrypt
```

Build and test (Go 1.26 is required by the indigo dependency; `GOTOOLCHAIN=auto` fetches it):

```console
$ cd age-atproto/age-plugin-atproto
$ GOTOOLCHAIN=auto go test ./...
$ GOTOOLCHAIN=auto go install .    # puts age-plugin-atproto on PATH
```

Dependencies: `filippo.io/age` (plugin framework, native recipients) and `github.com/bluesky-social/indigo` (handle/DID resolution with SSRF protection, CAR loading, MST, commit signature verification, DAG-CBOR). The verification logic in `resolve.go` is about 150 lines because indigo does the heavy lifting.

What was tested:

- All verification steps against a mock PLC directory and PDS that serve a real k256-signed commit, real MST, and real CAR.
- End to end with the installed `age` 1.1.1: `age -r age1atproto1...` produces a file that `age -d -i key.txt` decrypts without the plugin, and the plugin's status line reaches the terminal.
- `publish` against the mock PDS, followed by a verified lookup of what it wrote.

What was not tested, because this sandbox cannot reach `plc.directory`, `bsky.app`, or `maragu.dev`: live handle resolution over DNS/HTTPS, a real PDS's `sync.getRecord` output, and `publish` against a real PDS. These are the first things to try on a machine with network access:

```console
$ age-plugin-atproto lookup @markus.maragu.dev        # expect "has not published any age recipients"
$ age-plugin-atproto publish -identifier @markus.maragu.dev age1...
$ age-plugin-atproto lookup @markus.maragu.dev
$ echo Hello! | age -r "$(age-plugin-atproto recipient @markus.maragu.dev)" | age -d -i key.txt
```

## Decisions made without asking

These are the points where I would have asked a question first, with the choice I made.

1. **Record vs DID document:** record, for the reasons in the table. DID-document support is a clean add-on if you hold your own rotation key.
2. **One `self` record with an array vs one record per key:** one record, so the proof covers the whole key set.
3. **Which recipient types may be published:** native age only. Allowing plugin recipients or SSH keys is a one-line change each, but I think both are wrong defaults.
4. **The exact CLI shape:** stock-compatible `age1atproto1...` recipients plus `recipient`, `lookup`, and `publish` subcommands, and a shell function for `-r @handle`. No fork of age.
5. **Lexicon NSID `dev.maragu.age.recipients`:** your domain, since it is your prototype. Easy to rename.
6. **No pinning, caching, or freshness cross-check yet.** Listed under nuances; each is a contained follow-up.
7. **App-password auth for `publish`.** Good enough for a CLI prototype; OAuth later.

## Sources

age

- [age plugin specification (c2sp.org/age-plugin)](https://github.com/C2SP/C2SP/blob/main/age-plugin.md) and the [age format spec](https://github.com/C2SP/C2SP/blob/main/age.md)
- [`filippo.io/age/plugin`](https://github.com/FiloSottile/age/tree/main/plugin): `plugin.go`, `client.go`, `encode.go`
- [`cmd/age/parse.go`](https://github.com/FiloSottile/age/blob/main/cmd/age/parse.go), recipient dispatch and the removed `github:` type
- [age Plugins (Filippo Valsorda)](https://words.filippo.io/age-plugins/)
- [Security considerations of using SSH keys for encryption (discussion #540)](https://github.com/FiloSottile/age/discussions/540)
- [awesome-age](https://github.com/FiloSottile/awesome-age) for the existing plugin landscape

ATproto

- Specs: [Handle](https://atproto.com/specs/handle), [DID](https://atproto.com/specs/did), [Cryptography](https://atproto.com/specs/cryptography), [Repository](https://atproto.com/specs/repository), [Sync](https://atproto.com/specs/sync), [Record Key](https://atproto.com/specs/record-key), [Lexicon](https://atproto.com/specs/lexicon), [Label](https://atproto.com/specs/label)
- [did:plc method specification v0.3](https://github.com/did-method-plc/did-method-plc/blob/main/website/spec/v0.1/did-plc.md) and the server's [`constraints.ts`](https://github.com/did-method-plc/did-method-plc/blob/main/packages/server/src/constraints.ts)
- [Relaxing DID PLC Verification Method Constraints, June 2025 (discussion #3928)](https://github.com/bluesky-social/atproto/discussions/3928)
- [Planned Changes to DID Documents, August 2023 (discussion #1510)](https://github.com/bluesky-social/atproto/discussions/1510)
- [Encryption for private content (discussion #121)](https://github.com/bluesky-social/atproto/discussions/121)
- [`com.atproto.sync.getRecord`](https://github.com/bluesky-social/atproto/blob/main/lexicons/com/atproto/sync/getRecord.json), [`identity.signPlcOperation`](https://github.com/bluesky-social/atproto/blob/main/lexicons/com/atproto/identity/signPlcOperation.json)
- [indigo](https://github.com/bluesky-social/indigo): `atproto/identity`, `atproto/repo`, `atproto/atcrypto`, `atproto/atdata`
- [Introspecting Public Keys in ATProto (Kshitij Chauhan)](https://www.haroldadmin.com/posts/introspecting-public-keys-in-atproto)
- [Under Bluesky's hood: the ATProto](https://a-cup-of.coffee/blog/atproto-pds/), independent record verification walkthrough
- [multicodec table](https://github.com/multiformats/multicodec/blob/master/table.csv) (`x25519-pub` = `0xec`)

Prior art

- [Tangled `sh.tangled.publicKey` lexicon](https://tangled.org/tangled.org/core/blob/master/lexicons/publicKey.json) and [atmo.sh](https://atmo.sh/), SSH keys as atproto records
- [Bluesky SSH Authentication (tunbury.org)](https://www.tunbury.org/2025/04/25/bluesky-ssh-authentication/), `AuthorizedKeysCommand` from atproto records
- [Germ DM on ATProto](https://www.germnetwork.com/blog/germdm-atproto-now-beta), MLS-based E2EE messaging integrated with atproto identity
