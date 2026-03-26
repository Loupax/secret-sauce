# Changelog

All notable changes to this project will be documented here.

---

## v1.0.0 — The Sauce Release

### Highlights

- **Command renamed: `secret-sauce` → `sauce`** — The binary is now invoked as `sauce`.
  This is the defining milestone of the v1 release. All shell examples, help text, and
  documentation have been updated accordingly.

- **New environment variable: `SAUCE_DIR`** — The primary vault directory override is now
  `SAUCE_DIR`. The legacy `SECRET_SAUCE_DIR` variable remains fully supported as a
  transparent fallback. No changes to existing shell configs or CI pipelines are required.

- **IPC socket renamed** — The daemon socket is now `sauce.sock`
  (`$XDG_RUNTIME_DIR/sauce.sock`, falling back to `/tmp/sauce-<uid>.sock`).
  Any daemon started by a previous version of the binary must be stopped and restarted
  after upgrading.

- **Vault and config paths are unchanged** — `~/.local/share/secret-sauce` and
  `~/.config/secret-sauce` are preserved as-is. No file migration is required.

---

> **Previous development history**
> The entries below describe pre-v1 development progress on the `main` branch.

---

## [Unreleased — pubkey-derivation]

### Added

- **`share pubkey` command** — derives and prints the user's `age` public key on demand,
  so it can be shared with teammates for use with `share add`. Uses `resolveService()` to
  route through the daemon when available, or the OS keyring directly when not.

---

## [Unreleased — secret-types]

### Breaking Changes

- **`set` now requires a type argument** — the command signature has changed from
  `set <key> <value>` to `set <type> <key> <value>`. The type must be one of
  `environment` or `file`. Any existing scripts calling `set` must be updated.

- **`ls` output format changed** — output is now tab-separated `<type>\t<key>` per line
  instead of just `<key>`. Scripts that parse `ls` output must be updated.

### Added

- **Secret types** — secrets now carry an explicit type field stored in the encrypted
  envelope:
  - `environment` — injected as environment variables by `run`.
  - `file` — stored encrypted but not injected into the subprocess environment.

- **`edit <type> <key>` command** — opens the current value of a secret in `$EDITOR`
  (falls back to `vi`, then `nano`). On clean editor exit the updated content is
  re-encrypted and persisted. A non-zero editor exit code leaves the vault unchanged.
  The decrypted value is written to a `0600` temp file and cleaned up with `defer`.

- **Shell autocompletion** via Cobra's `ValidArgsFunction` for `set`, `edit`, and `rm`:
  - `set` — first argument completes to `environment`/`file`; second argument completes
    to existing keys.
  - `edit` — same argument positions as `set`.
  - `rm` — first argument completes to existing keys.

### Added (continued)

- **Ghost File injection for `file` secrets** — `run` now materializes `file`-typed
  secrets as unlinked, memory-backed file descriptors and injects them into the child
  process as `KEY=/dev/fd/N` environment variables. The lifecycle is:
  1. `os.CreateTemp` creates a file; `os.Remove` immediately unlinks it from the
     filesystem, making it invisible to all other processes while keeping the inode alive
     in RAM via the open file descriptor held by `secret-sauce`.
  2. The secret value is written into the in-memory fd; the cursor is seeked back to 0.
  3. The fd is passed to the child via `exec.Cmd.ExtraFiles`; Go maps `ExtraFiles[i]` to
     child fd `3+i`, so the formula `fdIndex = 3 + len(ExtraFiles) - 1` gives the correct
     descriptor number after each append.
  4. A `defer` loop closes all extra file descriptors on parent exit, causing the kernel
     to immediately reclaim the unlinked inode. The secret never exists as a linked,
     discoverable file on disk.

### Changed

- **`run` handles both secret types** — `environment` secrets continue to be merged as
  plain `KEY=VALUE` pairs; `file` secrets are now injected via the Ghost File pattern
  (see above) rather than being silently dropped.

- **`VaultService` interface extended** — `WriteSecret` now accepts a `vault.SecretType`
  parameter; `ReadAllSecrets` and the new `ReadSecret` return `vault.SecretInfo` (carrying
  both `Type` and `Value`) instead of bare strings.

- **IPC wire protocol updated** — `OpReadAll` responses now carry `map[string]SecretMeta`
  (with `type` and `value` fields) instead of `map[string]string`. A new `OpReadOne` op
  supports single-key reads.

---

## [Unreleased]

### Added

- **Hybrid daemon / fallback execution model** — commands that require the private key
  (`run`, `set`, `rm`) now resolve their execution path dynamically:
  1. If the Unix socket (`$XDG_RUNTIME_DIR/secret-sauce.sock`) is responsive, the request
     is sent to the background daemon over IPC.
  2. If the socket is absent and `auto_spawn: true`, the CLI spawns a detached daemon
     process (`syscall.Setsid`), waits for it to become ready, then uses IPC.
  3. If the socket is absent and `auto_spawn: false`, the CLI falls back to querying the
     OS keyring directly in the foreground — identical to prior behaviour.

- **`daemon start` / `stop` / `status` commands** — explicit lifecycle management for the
  background daemon process.

- **Background daemon server** (`internal/daemon`) — listens on a `0600` Unix Domain
  Socket, caches the `age` private key after its first keyring access, and handles
  `read_all`, `write`, `delete`, `ping`, and `shutdown` IPC operations. Shuts down
  automatically after the configured idle timeout.

- **Idle timeout** — the daemon resets a timer on every request. If no request arrives
  within the timeout period (default `15m`), the daemon removes the socket and exits.

- **`VaultService` interface** (`internal/service`) — strategy pattern abstraction over
  the two execution backends. `LocalVaultService` calls the crypto and keyring packages
  directly; `IPCVaultService` marshals requests over the Unix socket. Commands accept
  whichever implementation `resolveService()` returns.

- **Configuration file** (`~/.config/secret-sauce/config.json`) — supports `timeout`
  (Go duration string) and `auto_spawn` (boolean). Defaults to `{"timeout":"15m","auto_spawn":true}`
  when the file is absent.

- **IPC protocol** (`internal/ipc`) — newline-delimited JSON request/response over a
  Unix Domain Socket. Socket path: `$XDG_RUNTIME_DIR/secret-sauce.sock`, falling back to
  `/tmp/secret-sauce-<uid>.sock` when `XDG_RUNTIME_DIR` is unset.

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
- Integration tests for IPC and daemon lifecycle.
- Shell completion scripts.
- Pre-built binaries / install script.
