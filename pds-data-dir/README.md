# What the Bluesky PDS writes to its data directory

Question: if a PDS (https://github.com/bluesky-social/pds) runs on a VPS that
is stateless except for persistent storage, what exactly has to live on that
storage? Is it only SQLite databases and blobs?

Answer: no. Besides SQLite and blobs, the PDS writes **raw private key files**
into the data directory, and those are the one thing you cannot regenerate.

Tested with `@atproto/pds` 0.5.27 (distro version 0.4.5027, the `service/`
directory of the pds repo at 2026-09-07), run directly with Node 22 against a
local mock PLC directory, with `strace` recording every file-creating syscall.
No file was written anywhere outside `PDS_DATA_DIRECTORY` (`$HOME` and the
working directory stayed empty).

## Files created

Paths are relative to `PDS_DATA_DIRECTORY` (`/pds` in the standard install).
`<hh>` is the first two hex chars of sha256(did).

| Path | What it is | Regenerable? |
|---|---|---|
| `account.sqlite` (+ `-wal`, `-shm`) | Accounts, credentials, invite codes, OAuth state, takedowns | No |
| `sequencer.sqlite` (+ `-wal`, `-shm`) | Firehose event log and sequence numbers | No (relays would see a cursor reset) |
| `did_cache.sqlite` (+ `-wal`, `-shm`) | Cache of resolved DID documents | Yes, it is a cache |
| `actors/<hh>/<did>/store.sqlite` (+ `-wal`, `-shm`, transient `-journal`) | The account's repo: records, MST blocks, blob refs | No |
| `actors/<hh>/<did>/key` | **32 raw bytes: the account's secp256k1 repo signing private key** | No |
| `actors/reserved_keys/<did:key>` | **32 raw bytes: a signing key reserved via `com.atproto.server.reserveSigningKey`** before an account migrates in | No, until the migration completes |
| `blocks/<did>/<cid>` | Blobs referenced by a committed record | Content-addressed, but not re-fetchable |
| `blocks/tempt/<did>/<random>` | Uploaded blobs not yet referenced by a record | Orphans are never cleaned up |
| `blocks/quarantine/<did>/<cid>` | Blobs moved out of `blocks/<did>/` by an admin takedown | Same as blobs |

The path names come from `config/config.js`, `actor-store/actor-store.js` and
`disk-blobstore.js` in the package. `PDS_ACCOUNT_DB_LOCATION`,
`PDS_SEQUENCER_DB_LOCATION`, `PDS_DID_CACHE_DB_LOCATION`,
`PDS_ACTOR_STORE_DIRECTORY`, `PDS_BLOBSTORE_DISK_LOCATION`,
`PDS_BLOBSTORE_DISK_TMP_LOCATION` override each location individually. The
`key` file always sits next to `store.sqlite`; it cannot be relocated on its
own. Blobs can go to S3 instead of disk.

`actor-store.js` still contains code to write a `did-op` file next to the
key, but nothing in 0.5.27 calls it.

## Observed lifecycle

- Startup with an empty directory creates the three top-level SQLite files.
- `createAccount` creates `actors/<hh>/<did>/{key,store.sqlite}` and POSTs
  the genesis operation to the PLC directory.
- `uploadBlob` writes to `blocks/tempt/<did>/`. The file moves to
  `blocks/<did>/<cid>` only when a record referencing it is committed. An
  upload that is never referenced stays in `tempt` forever.
- Admin takedown of a blob renames it into `blocks/quarantine/<did>/`.
- `reserveSigningKey` writes `actors/reserved_keys/<did:key>`. It is removed
  only when `createAccount` consumes it.
- Admin `deleteAccount` removes the whole `actors/<hh>/<did>/` directory and
  the account's blob directories.

## What this means for a stateless VPS

1. Mount the whole data directory as persistent storage, or at least
   `account.sqlite`, `sequencer.sqlite`, `actors/` and `blocks/` (or S3 for
   blobs). Treat `actors/**/key` as secret material: it is unencrypted.
2. Secrets in `pds.env` are state too: `PDS_PLC_ROTATION_KEY_K256_PRIVATE_KEY_HEX`
   is the rotation key on every did:plc the server created, and
   `PDS_JWT_SECRET` and `PDS_DPOP_SECRET` invalidate sessions if they change.
3. The databases run in WAL mode. Copying `*.sqlite` alone while the server
   runs loses whatever is still in the `-wal` file. Use `sqlite3 .backup`,
   snapshot the volume, or stop the service first.
4. The default install also keeps `/pds/caddy/data` (TLS certificates and
   ACME account key) and `/pds/caddy/etc/caddy/Caddyfile`. Certificates are
   regenerable; losing them just triggers re-issuance, subject to Let's
   Encrypt rate limits.

## Reproducing

```sh
git clone https://github.com/bluesky-social/pds && cd pds/service
corepack enable && pnpm install --production --frozen-lockfile
node mock-plc.mjs &                  # from this directory, listens on :2582
export PDS_HOSTNAME=localhost PDS_DEV_MODE=true PDS_PORT=3000 \
  PDS_JWT_SECRET=$(openssl rand -hex 16) PDS_ADMIN_PASSWORD=admin \
  PDS_PLC_ROTATION_KEY_K256_PRIVATE_KEY_HEX=$(openssl rand -hex 32) \
  PDS_DATA_DIRECTORY=$PWD/data PDS_BLOBSTORE_DISK_LOCATION=$PWD/data/blocks \
  PDS_DID_PLC_URL=http://localhost:2582 PDS_INVITE_REQUIRED=false \
  PDS_RATE_LIMITS_ENABLED=false PDS_DISABLE_SSRF_PROTECTION=true \
  PDS_BSKY_APP_VIEW_URL=https://api.bsky.app PDS_BSKY_APP_VIEW_DID=did:web:api.bsky.app
strace -f -o strace.log -e trace=openat,mkdir,rename,unlink,unlinkat \
  node --enable-source-maps index.ts
```

Then create an account, upload a blob, create a record embedding it, call
`reserveSigningKey`, and run `find data`. The mock PLC accepts any
operation and serves back a DID document derived from it, so nothing is
registered with the real plc.directory.
