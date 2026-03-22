# Changelog

All notable changes to this project will be documented here.

> **This project has not yet reached a stable release.**
> There is no versioned release. All entries below describe development progress on
> the `main` branch. Breaking changes may occur at any time without a major version bump.

---

## [Unreleased]

### Added

- **`init` command** — generates an X25519 keypair via `filippo.io/age`, stores the
  private key in the OS keyring (Linux Secret Service / D-Bus), writes the public key
  as the first entry in `vault_recipients.txt`, and writes an empty encrypted vault.
- **`set KEY VALUE` command** — acquires an exclusive file lock, decrypts the vault,
  upserts the key/value pair, and re-encrypts to all current recipients.
- **`rm KEY` command** — same flow as `set`; returns an error if the key does not exist.
- **`ls` command** — acquires a shared file lock, decrypts the vault, and prints key
  names in alphabetical order. Values are never printed.
- **`run -- <cmd>` command** — decrypts the vault into memory, merges secrets into
  `os.Environ()`, and executes the child command with the combined environment. Proxies
  stdin/stdout/stderr and preserves the child's exit code.
- **`share add <pubkey>` command** — validates the provided `age1...` public key,
  appends it to `vault_recipients.txt`, and re-encrypts the vault to all recipients.
- **`share ls` command** — prints all public keys in `vault_recipients.txt`.
- **`--vault-dir` flag** — overrides the vault directory for any command.
- **`$SECRET_SAUCE_DIR` env var** — alternative vault directory override (lower
  priority than the flag).
- **XDG base directory support** — defaults to `$XDG_DATA_HOME/secret-sauce/`
  (typically `~/.local/share/secret-sauce/`).
- **Multi-recipient `age` envelope encryption** — any write operation re-encrypts to
  all keys in `vault_recipients.txt`, enabling transparent team secret sharing.
- **OS-level file locking** — `flock(2)` on `vault.lock` prevents concurrent writers
  from corrupting the vault. Readers acquire a shared lock; writers acquire an
  exclusive lock.
- **Atomic vault writes** — secrets are written to a temp file, synced, and renamed
  into place. Partial writes do not corrupt the live vault.
- **Graceful D-Bus error handling** — if no Secret Service provider is running (common
  on minimal Wayland compositors like Sway), the tool prints an actionable error
  message naming specific providers to start (`keepassxc`, `gnome-keyring-daemon`)
  rather than panicking or emitting a raw library error.
- **`internal/keyring` package** — thin wrapper over `go-keyring` with D-Bus error
  detection and the `ErrNoSecretService` sentinel.
- **`internal/vault` package** — age encrypt/decrypt, JSON serialisation, file locking,
  and recipient manifest management.
- **Unit tests** for `internal/keyring` (mock keyring backend, D-Bus sentinel
  detection) and `internal/vault` (init/read/write round-trip, multi-recipient
  encryption, concurrent access).

### Not yet implemented

- Recipient removal.
- Private key rotation.
- Full vault deletion / re-initialisation helper.
- Export / import / backup commands.
- End-to-end integration tests against a real Secret Service daemon.
- Shell completion scripts.
- Pre-built binaries / install script.
