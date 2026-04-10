# secret-sauce
<img width="200" alt="logo" src="./assets/logo.png" />

_The secret is in the sauce_

A local-first, multi-user CLI secret manager for Linux. Secrets are stored on disk as
individual [`age`](https://age-encryption.org/)-encrypted files and injected as environment
variables into a child process. Sharing is handled by re-encrypting each secret to
multiple `age` X25519 recipients — no server, no cloud, no central authority.

---

## How it works

`sauce` maintains a vault directory (default `~/.local/share/secret-sauce/`):

| Path | Contents |
|---|---|
| `.vault_recipients` | Plaintext list of authorised `age` public keys (one per line) |
| `<KEY>.age` | One `age`-encrypted file per secret, named after the secret key |
| `vault.lock` | Transient file used for `flock`-based concurrency control |

Your private key is generated once at `init` time and stored in the OS keyring via the
[Linux Secret Service API](https://specifications.freedesktop.org/secret-service/) (D-Bus).
On Sway and other minimal Wayland compositors, a provider such as KeePassXC or
`gnome-keyring-daemon` must be running for the keyring to be available.

Each secret is encrypted independently to all recipients listed in `.vault_recipients`.
Adding a recipient (`share add`) re-encrypts every secret file to the updated list.

Secrets are decrypted concurrently when running a command, keeping startup overhead low
even for large vaults.

File-level locking (`flock`) prevents concurrent writers from corrupting the vault.

### Hybrid execution model

Commands that need the private key (`run`) choose their execution path dynamically:

1. **Daemon available** — the request is sent over a Unix Domain Socket to the background
   daemon, which has the private key cached in memory. No D-Bus prompt is fired.
2. **No daemon + `auto_spawn: true`** (default) — the CLI automatically spawns a detached
   daemon, waits for it to be ready, then sends the request over IPC.
3. **No daemon + `auto_spawn: false`** — the CLI queries the OS keyring directly in the
   foreground (same behaviour as before the daemon was introduced).

The daemon idles out after a configurable timeout (default 15 minutes) and removes its
socket on shutdown.

---

## Requirements

- Linux (x86-64 or ARM64)
- Go 1.25+ (to build from source)
- A running [Secret Service](https://specifications.freedesktop.org/secret-service/)
  provider on D-Bus:
  - **KeePassXC** — enable *Tools → Settings → Secret Service Integration*
  - **GNOME Keyring** — usually running automatically in GNOME sessions; start manually
    with `/usr/lib/gnome-keyring-daemon --start`
  - **KWallet** (KDE) — supported via the Secret Service bridge

> The daemon only needs to contact the Secret Service **once** at startup. After that,
> D-Bus prompts are bypassed for the lifetime of the daemon session.

---

## Installation

**From a cloned repo (recommended):**

```bash
git clone https://github.com/loupax/secret-sauce
cd secret-sauce
go install ./cmd/sauce
```

This installs the `sauce` binary to `$(go env GOPATH)/bin`.
Make sure that directory is on your `$PATH` (it usually is if you have Go installed).

**Without cloning, once a release is published:**

```bash
go install github.com/loupax/secret-sauce/cmd/sauce@latest
```

> Requires a published release tag.

**Build a local binary only (no PATH change):**

```bash
go build -o sauce ./cmd/sauce
```

---

## Usage

### Initialise a vault

```bash
sauce init
```

Generates a fresh X25519 keypair. The private key is stored in the OS keyring. The
public key is printed to stdout — keep it handy if you want to be added as a recipient
on a teammate's vault.

```
Vault initialized.
Public key (share this with teammates): age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
```

### Secret types

Secrets have an explicit type that controls how they are consumed:

| Type | Description |
|---|---|
| `environment` | Injected as an environment variable when running `sauce run` |
| `file` | Materialized as a memory-backed ghost file and injected as `KEY=/dev/fd/N`; the file has no filesystem path and is invisible to other processes |
| `map` | A flat `map[string]string` stored as JSON. **Never injected** by `run`; accessed on-demand via `sauce get` |

### Add / update a secret

```bash
sauce set environment DATABASE_URL "postgres://user:pass@localhost/mydb"
sauce set environment API_KEY "sk-..."
sauce set file TLS_CERT "$(cat server.crt)"
```

The first argument is the type (`environment`, `file`, or `map`). `environment` secrets are injected as plain environment variables. `file` secrets are injected as memory-backed ghost files (see below).

### Add / update a map secret

```bash
# From a JSON literal
sauce set map CREDENTIALS '{"user":"alice","token":"s3cr3t"}'

# Interactively (values are masked at input)
sauce set map CREDENTIALS --interactive
```

A `map` secret stores a flat `map[string]string` as encrypted JSON. All values must be strings — nested objects and arrays are rejected. Keys may be any valid string. Map secrets are **never** automatically injected by `sauce run`; use `sauce get` to retrieve them explicitly.

### Get a secret value

```bash
# Print the full value (any type)
sauce get DATABASE_URL
sauce get TLS_CERT

# Print a specific key from a map secret (no trailing newline — suitable for shell substitution)
sauce get CREDENTIALS user
TOKEN=$(sauce get CREDENTIALS token)
```

`sauce get` works for all secret types. For `map` secrets a second argument selects a specific key; for `environment` and `file` secrets the full value is printed.

### Edit a secret in your editor

```bash
sauce edit environment DATABASE_URL
sauce edit file TLS_CERT
```

Opens the current value in `$EDITOR` (falls back to `vi`, then `nano`). When the editor
exits cleanly, the updated content is re-encrypted and persisted. If the editor exits with
a non-zero code, the vault is left unchanged.

### Remove a secret

```bash
sauce rm API_KEY
```

Returns an error if the key does not exist.

### List secret keys

```bash
sauce ls
```

Prints tab-separated `<type>\t<key>` lines, sorted alphabetically by key. Values are
never output to the terminal. The tab-separated format is suitable for UNIX pipeline
composition:

```
environment	API_KEY
environment	DATABASE_URL
file	TLS_CERT
```

### Run a command with secrets injected

```bash
sauce run -- env | grep DATABASE_URL
sauce run -- python manage.py runserver
sauce run -- bash -c 'echo $DATABASE_URL'
```

Decrypts all secrets concurrently into memory, then executes the given command with the
combined environment. Standard I/O is proxied transparently and the child's exit code is
preserved. Uses the daemon if available (see below), otherwise falls back to querying the
keyring directly.

**`environment` secrets** are merged into the child's environment as plain `KEY=VALUE`
pairs, identical to regular environment variables.

**`file` secrets** are injected using the Ghost File pattern:

1. A temporary file is created on disk and immediately **unlinked** — the directory entry
   is removed, making the file invisible to `ls`, `find`, and any other process. The
   file's inode remains alive in RAM only because `sauce` holds an open file
   descriptor to it.
2. The secret's value is written into the in-memory file descriptor.
3. The child process receives `KEY=/dev/fd/N` in its environment, where `N` is the file
   descriptor number (3 or higher, since 0–2 are stdin/stdout/stderr).
4. The child reads the secret by opening the path in `$KEY` like any ordinary file:

   ```bash
   # shell
   openssl verify -CAfile "$TLS_CA" cert.pem

   # python
   with open(os.environ["TLS_CERT"]) as f:
       cert_pem = f.read()
   ```

5. When `sauce` exits (normally or on error), the kernel drops all file
   descriptors and instantly reclaims the inode. The secret never touches disk in a
   linked, discoverable form.

> **Linux only.** The `/dev/fd/N` interface is Linux-specific. This feature will not
> work on macOS or Windows.

### Manage the daemon

```bash
# Start the background daemon (detaches from the current session)
sauce daemon start

# Check whether the daemon is running
sauce daemon status

# Shut the daemon down gracefully
sauce daemon stop
```

The daemon caches the private key in memory after its first keyring access. Subsequent
`run` calls use IPC over a Unix socket
(`$XDG_RUNTIME_DIR/sauce.sock`) and never trigger a D-Bus prompt.

The daemon shuts itself down after the idle timeout (default `15m`) with no activity,
zeroes the socket, and exits cleanly. With `auto_spawn: true` (the default), the next
`run` call will start a fresh daemon automatically.

### Manage recipients (multi-user sharing)

```bash
# Print your own public key (share this with teammates so they can run 'share add')
sauce share pubkey

# Add a teammate by their public key
sauce share add age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p

# List all authorised public keys
sauce share ls
```

After `share add`, every secret file is re-encrypted to all recipients listed in
`.vault_recipients`. The new recipient can now decrypt secrets using their own private key
(which they initialised with `sauce init` in the same vault directory, typically
shared via rsync, a git repo, or a network filesystem).

### Import secrets from 1Password

```bash
sauce import 1password /path/to/export.1pux
```

Reads a 1Password Unencrypted Export (`.1pux`) file and imports all items as
secrets into the current vault.

> **CAUTION:** `.1pux` files are unencrypted plaintext. Delete the export file
> immediately after import.

| Item category | Secret type | Notes |
|---|---|---|
| `login` | `map` | Usernames, passwords, and custom section fields stored as a flat JSON map |
| `password` | `environment` | Password field value used; falls back to first non-empty login field |
| `document` | `file` | Raw file bytes stored base64-encoded |
| `database`, `server` | `map` | Section fields stored as a flat JSON map |
| anything else | `environment` | First non-empty field value used |

Document secrets are stored as base64-encoded strings. To recover the original
bytes, pipe through `base64 --decode`:

```bash
sauce get MY_CERT | base64 --decode > server.crt
```

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--concurrency N` | `0` (auto) | Maximum number of parallel write operations. `0` falls through to the `concurrency` config field, then `runtime.NumCPU()` |

The `concurrency` option can also be set permanently in
`~/.config/secret-sauce/config.json`:

```json
{
  "concurrency": 4
}
```

---

## Vault directory

The vault directory is resolved in this order:

1. `--vault-dir <path>` flag
2. `$SAUCE_DIR` environment variable
3. `$SECRET_SAUCE_DIR` environment variable *(legacy fallback — still supported)*
4. `$XDG_DATA_HOME/secret-sauce/` (default: `~/.local/share/secret-sauce/`)

> **Upgrading from an earlier version?** Vault and config paths are unchanged — no file migration needed.

For shared-team use, point all team members at the same directory (e.g. a shared NFS
mount or a directory synced with rsync or git):

```bash
export SAUCE_DIR=/mnt/team-share/secrets
```

Because each secret is a separate file, syncing tools like `rsync` or `git` can merge
changes from multiple machines without last-write-wins clobbering.

---

## Configuration

`~/.config/secret-sauce/config.json` (created automatically with defaults if absent):

```json
{
  "timeout": "15m",
  "auto_spawn": true
}
```

| Field | Default | Description |
|---|---|---|
| `timeout` | `"15m"` | Idle period after which the daemon shuts itself down |
| `auto_spawn` | `true` | Automatically start the daemon when a command needs it |
| `concurrency` | `0` (auto) | Max parallel write operations for `import`; `0` uses `runtime.NumCPU()` |

Set `auto_spawn: false` to always query the keyring directly without a daemon.

---

## Security model

- **Protection goal:** secrets at rest and during synchronisation.
- **Accepted risk:** if your session is unlocked and an attacker has access to your
  keyboard or can run processes as your user, they can decrypt the vault. The tool does
  not defend against an attacker with local session access.
- **Private keys** never touch disk — they live only in the OS keyring and in process
  memory during an operation (or in the daemon's memory while it is running).
- **Daemon socket** is created with `0600` permissions, restricting access to the
  owning user only.
- **Values** are never written to stdout; `ls` prints only key names.
- **Temp files** are written inside the vault directory and atomically renamed into
  place; partial writes do not corrupt live secret files.

---

## Project structure

```
secret-sauce/
├── cmd/
│   ├── sauce/                # binary entry point (go install ./cmd/sauce → sauce)
│   │   └── main.go
│   ├── root.go               # vault directory resolution, persistent flags
│   ├── init.go
│   ├── get.go
│   ├── set.go
│   ├── edit.go
│   ├── rm.go
│   ├── ls.go
│   ├── run.go
│   ├── share.go
│   ├── daemon.go             # daemon start / stop / status commands
│   ├── import.go             # sauce import 1password command
│   └── service_resolver.go   # hybrid execution decision tree
└── internal/
    ├── config/               # config.json loading with defaults
    ├── ipc/                  # Unix socket protocol (request/response types)
    ├── daemon/               # daemon server (idle timeout, graceful shutdown)
    ├── service/              # VaultService interface + Local and IPC implementations
    ├── keyring/              # OS keyring wrapper (go-keyring + D-Bus error handling)
    └── vault/                # age encryption, file locking, recipient management
        ├── lock.go
        ├── recipients.go
        └── vault.go
```

---

## Known limitations (pre-alpha)

- No `delete` command for removing the entire vault.
- No `export` command for backup.
- No way to remove a recipient without re-initialising the vault.
- The private key cannot be rotated without re-initialising.
- No support for secret namespacing or tagging.
- End-to-end tests against a real Secret Service daemon are not yet implemented.
- Windows and macOS are not supported (and not a goal).

---

## Dependencies

| Package | Purpose |
|---|---|
| [`filippo.io/age`](https://pkg.go.dev/filippo.io/age) | X25519 key generation, multi-recipient envelope encryption |
| [`github.com/spf13/cobra`](https://github.com/spf13/cobra) | CLI framework |
| [`github.com/zalando/go-keyring`](https://github.com/zalando/go-keyring) | Linux Secret Service API (D-Bus) |
| [`golang.org/x/sys`](https://pkg.go.dev/golang.org/x/sys) | `flock` for OS-level file locking |
| [`golang.org/x/sync`](https://pkg.go.dev/golang.org/x/sync) | `errgroup` for concurrent secret decryption |
| [`golang.org/x/term`](https://pkg.go.dev/golang.org/x/term) | Terminal password masking for `sauce set map --interactive` |
| Go standard library `net` | Unix Domain Socket IPC between CLI client and daemon |

---

## License

TBD
