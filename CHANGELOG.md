# Changelog

All notable changes to this project will be documented here.

> **This project has not yet reached a stable release.**
> There is no versioned release. All entries below describe development progress on
> the `main` branch. Breaking changes may occur at any time without a major version bump.

---

## [Unreleased]

### Changed

- **Directory-as-vault storage** — the vault is now a directory of individual
  `<KEY>.age` files rather than a single monolithic encrypted blob. Each secret is
  stored as its own `age`-encrypted file, enabling safe distributed syncing via `rsync`
  or `git` without last-write-wins clobbering.
- **`.vault_recipients`** replaces `vault_recipients.txt` as the recipient manifest filename.
- **`set` command** — writes only the affected secret file; no longer reads and
  re-encrypts the entire vault.
- **`rm` command** — deletes the individual `<KEY>.age` file; no longer reads and
  re-encrypts the entire vault.
- **`ls` command** — lists secret keys by reading filenames; no decryption required.
- **`run` command** — decrypts all `<KEY>.age` files concurrently using
  `golang.org/x/sync/errgroup` before merging into the child environment.
- **`share add` command** — re-encrypts each secret file individually to the updated
  recipient list.
- **`share ls` command** — reads `.vault_recipients` instead of `vault_recipients.txt`.
- **`init` command** — initialises the vault directory and writes `.vault_recipients`;
  no longer writes an empty encrypted vault file.

### Added

- **`init` command** — generates an X25519 keypair via `filippo.io/age`, stores the
  private key in the OS keyring (Linux Secret Service / D-Bus), writes the public key
  as the first entry in `.vault_recipients`.
- **`set KEY VALUE` command** — acquires an exclusive file lock and encrypts the value
  to `<KEY>.age` for all current recipients.
- **`rm KEY` command** — acquires an exclusive file lock and removes `<KEY>.age`.
  Returns an error if the key does not exist.
- **`ls` command** — acquires a shared file lock and prints key names in alphabetical
  order by reading filenames. Values are never printed.
- **`run -- <cmd>` command** — decrypts all secrets concurrently into memory, merges
  them into `os.Environ()`, and executes the child command with the combined
  environment. Proxies stdin/stdout/stderr and preserves the child's exit code.
- **`share add <pubkey>` command** — validates the provided `age1...` public key,
  appends it to `.vault_recipients`, and re-encrypts every secret file to all recipients.
- **`share ls` command** — prints all public keys in `.vault_recipients`.
- **`--vault-dir` flag** — overrides the vault directory for any command.
- **`$SECRET_SAUCE_DIR` env var** — alternative vault directory override (lower
  priority than the flag).
- **XDG base directory support** — defaults to `$XDG_DATA_HOME/secret-sauce/`
  (typically `~/.local/share/secret-sauce/`).
- **Multi-recipient `age` envelope encryption** — each secret file is encrypted to all
  keys in `.vault_recipients`, enabling transparent team secret sharing.
- **Concurrent decryption** — `run` decrypts all secret files in parallel via
  `golang.org/x/sync/errgroup`; the result map is assembled safely with a `sync.Mutex`.
- **OS-level file locking** — `flock(2)` on `vault.lock` prevents concurrent writers
  from corrupting secrets. Readers acquire a shared lock; writers acquire an exclusive lock.
- **Atomic secret writes** — each secret is written to a temp file, synced, and renamed
  into place. Partial writes do not corrupt the live file.
- **Graceful D-Bus error handling** — if no Secret Service provider is running (common
  on minimal Wayland compositors like Sway), the tool prints an actionable error
  message naming specific providers to start (`keepassxc`, `gnome-keyring-daemon`)
  rather than panicking or emitting a raw library error.
- **`internal/keyring` package** — thin wrapper over `go-keyring` with D-Bus error
  detection and the `ErrNoSecretService` sentinel.
- **`internal/vault` package** — per-secret age encrypt/decrypt, atomic file writes,
  file locking, and recipient manifest management.
- **Unit tests** for `internal/keyring` (mock keyring backend, D-Bus sentinel
  detection) and `internal/vault` (init/exists, write/read round-trip, delete,
  read-all-secrets, multi-recipient encryption).

### Not yet implemented

- Recipient removal.
- Private key rotation.
- Full vault deletion / re-initialisation helper.
- Export / import / backup commands.
- End-to-end integration tests against a real Secret Service daemon.
- Shell completion scripts.
- Pre-built binaries / install script.
