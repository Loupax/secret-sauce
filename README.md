# secret-sauce

> **STATUS: PRE-ALPHA — NOT READY FOR USE**
> This project is under active development. The CLI surface, storage format, and key
> management behaviour may change without notice between commits. Do not use this to
> store secrets you cannot afford to lose or rotate.

A local-first, multi-user CLI secret manager for Linux. Secrets are stored on disk as
individual [`age`](https://age-encryption.org/)-encrypted files and injected as environment
variables into a child process. Sharing is handled by re-encrypting each secret to
multiple `age` X25519 recipients — no server, no cloud, no central authority.

---

## How it works

`secret-sauce` maintains a vault directory (default `~/.local/share/secret-sauce/`):

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

---

## Installation

```bash
git clone https://github.com/loupax/secret-sauce
cd secret-sauce
go build -o secret-sauce .
# move the binary somewhere on your PATH
mv secret-sauce ~/.local/bin/
```

---

## Usage

### Initialise a vault

```bash
secret-sauce init
```

Generates a fresh X25519 keypair. The private key is stored in the OS keyring. The
public key is printed to stdout — keep it handy if you want to be added as a recipient
on a teammate's vault.

```
Vault initialized.
Public key (share this with teammates): age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
```

### Add / update a secret

```bash
secret-sauce set DATABASE_URL "postgres://user:pass@localhost/mydb"
secret-sauce set API_KEY "sk-..."
```

### Remove a secret

```bash
secret-sauce rm API_KEY
```

Returns an error if the key does not exist.

### List secret keys

```bash
secret-sauce ls
```

Prints key names only — values are never output to the terminal.

### Run a command with secrets injected

```bash
secret-sauce run -- env | grep DATABASE_URL
secret-sauce run -- python manage.py runserver
secret-sauce run -- bash -c 'echo $DATABASE_URL'
```

Decrypts all secrets concurrently into memory, merges them into the current environment,
then executes the given command with the combined environment. Standard I/O is proxied
transparently and the child's exit code is preserved.

### Manage recipients (multi-user sharing)

```bash
# Add a teammate by their public key
secret-sauce share add age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p

# List all authorised public keys
secret-sauce share ls
```

After `share add`, every secret file is re-encrypted to all recipients listed in
`.vault_recipients`. The new recipient can now decrypt secrets using their own private key
(which they initialised with `secret-sauce init` in the same vault directory, typically
shared via rsync, a git repo, or a network filesystem).

---

## Vault directory

The vault directory is resolved in this order:

1. `--vault-dir <path>` flag
2. `$SECRET_SAUCE_DIR` environment variable
3. `$XDG_DATA_HOME/secret-sauce/` (default: `~/.local/share/secret-sauce/`)

For shared-team use, point all team members at the same directory (e.g. a shared NFS
mount or a directory synced with rsync or git):

```bash
export SECRET_SAUCE_DIR=/mnt/team-share/secrets
```

Because each secret is a separate file, syncing tools like `rsync` or `git` can merge
changes from multiple machines without last-write-wins clobbering.

---

## Security model

- **Protection goal:** secrets at rest and during synchronisation.
- **Accepted risk:** if your session is unlocked and an attacker has access to your
  keyboard or can run processes as your user, they can decrypt the vault. The tool does
  not defend against an attacker with local session access.
- **Private keys** never touch disk — they live only in the OS keyring and in process
  memory during an operation.
- **Values** are never written to stdout; `ls` prints only key names.
- **Temp files** are written inside the vault directory and atomically renamed into
  place; partial writes do not corrupt live secret files.

---

## Project structure

```
secret-sauce/
├── main.go
├── cmd/                      # cobra command definitions
│   ├── root.go               # vault directory resolution, persistent flags
│   ├── init.go
│   ├── set.go
│   ├── rm.go
│   ├── ls.go
│   ├── run.go
│   └── share.go
└── internal/
    ├── keyring/              # OS keyring wrapper (go-keyring + D-Bus error handling)
    └── vault/                # age encryption, file locking, recipient management
        ├── lock.go
        ├── recipients.go
        └── vault.go
```

---

## Known limitations (pre-alpha)

- No `delete` command for removing the entire vault.
- No `export` / `import` commands for backup or migration.
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

---

## License

TBD
