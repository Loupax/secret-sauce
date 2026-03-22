# Security Audit Report

**Project:** secret-sauce
**Date:** 2026-03-22
**Scope:** Full codebase — focused on new daemon/IPC surface introduced in the hybrid-daemon PR
**Auditor:** Claude (automated), cross-referenced against `THREAT_MODEL.md`

---

## Executive Summary

The hybrid-daemon architecture is well-structured and the idle-timeout shutdown, socket-owner-only `chmod 0600`, and UDS transport are sound baseline choices. However, the IPC protocol has no authentication layer and treats the `VaultDir` field in every request as a trusted parameter, which any same-user process can supply arbitrarily. The most actionable finding is a narrow TOCTOU window on socket creation that is exploitable from any local user when the `/tmp` fallback path is in use. Additionally, passing secret values as command-line arguments to `secret-sauce set` exposes them via `/proc/<pid>/cmdline`.

---

## Findings

### FINDING-001: Socket Permission Race (TOCTOU)

| Field | Value |
|---|---|
| Severity | Medium |
| Category | Access Control & Filesystem Permissions |
| Location | `internal/daemon/server.go:37-48` |
| CWE | CWE-362 (Race Condition), CWE-732 (Incorrect Permission Assignment) |

**Description:** `net.Listen("unix", socketPath)` creates the socket file before `os.Chmod(0600)` narrows its permissions. During this window — however brief — the socket inherits the process umask, which is typically `0022`, leaving it group- and world-readable/writable. Any local user on the system can connect to the socket during this window and issue IPC requests including `OpReadAll`.

The risk is highest when using the `/tmp` fallback path (`/tmp/secret-sauce-<uid>.sock`), since `/tmp` is accessible to all users. When `$XDG_RUNTIME_DIR` is set (e.g., `/run/user/1000`, mode `0700`), other users cannot access the directory at all, making the window effectively unexploitable.

**Evidence:**
```go
// server.go:37-48
ln, err := net.Listen("unix", s.socketPath)  // socket created here, perms = 0777 & ~umask
if err != nil {
    return err
}
s.listener = ln

// Restrict socket to owner only — security requirement.
if err := os.Chmod(s.socketPath, 0600); err != nil {  // narrowed here — race window between these two lines
```

**Recommendation:** Set `umask(0177)` (via `syscall.Umask`) in the daemon process immediately before calling `net.Listen`, then restore it. This ensures the socket is created with `0600` from the start, eliminating the race entirely. Alternatively, create the socket inside a `0700` directory under the user's control.

---

### FINDING-002: IPC Protocol Accepts Arbitrary `VaultDir` Without Validation

| Field | Value |
|---|---|
| Severity | Medium |
| Category | Access Control |
| Location | `internal/daemon/server.go:86-106`, `internal/ipc/ipc.go:16-21` |
| CWE | CWE-22 (Path Traversal), CWE-284 (Improper Access Control) |

**Description:** Every IPC `Request` includes a `VaultDir` field, and the daemon passes it directly to `LocalVaultService` without validating that it matches the vault the daemon was started for. Any process that can connect to the socket can:

- Send `OpReadAll` with an arbitrary `VaultDir` — the daemon will attempt to load whatever keyring entry corresponds to `SHA-256(that_vaultDir)` and decrypt secrets from the specified directory.
- Send `OpWrite` with `vault_dir=/tmp` — the daemon will write encrypted files into `/tmp` (or any path the daemon process can write to). The `validateKey` function in `vault.go` only validates the key name, not the vault directory.
- Send `OpDelete` with `vault_dir=/home/user/projects` — the daemon will attempt to remove `<key>.age` from that path.

In practice this is exploitable only by processes running as the same user (the socket is `0600`), so the threat is a compromised co-resident process — consistent with the accepted risk in Threat 3 of the threat model. However, it meaningfully extends the attack surface beyond what was described there (previously, an attacker needed to query D-Bus directly; now they can query the socket with a broader set of operations, including writes and deletes to arbitrary paths).

**Evidence:**
```go
// ipc.go:16-21
type Request struct {
    Op       string `json:"op"`
    VaultDir string `json:"vault_dir"`   // no validation on receipt
    Key      string `json:"key,omitempty"`
    Value    string `json:"value,omitempty"`
}

// server.go:86-88
case ipc.OpReadAll:
    secrets, err := s.svc.ReadAllSecrets(req.VaultDir)   // vaultDir is caller-supplied
```

**Recommendation:** At daemon startup, record the canonical vault directory (or directories) it is authorized to serve. In `handleConn`, reject any request whose `VaultDir` does not match. At minimum, apply `filepath.Clean` and verify the path does not escape a known root. This converts the protocol from "trust the caller's path" to "daemon knows its own scope."

---

### FINDING-003: Unauthenticated `OpShutdown` — Daemon DoS

| Field | Value |
|---|---|
| Severity | Low |
| Category | Access Control |
| Location | `internal/daemon/server.go:108-111` |
| CWE | CWE-306 (Missing Authentication for Critical Function) |

**Description:** Any process running as the same user can send `OpShutdown` to terminate the daemon. There is no challenge, token, or capability check. This is a denial-of-service: a malicious or misbehaving co-resident process can kill the daemon on demand, forcing subsequent `secret-sauce run` calls to either re-spawn it (if `auto_spawn=true`) or fall back to direct D-Bus, adding latency.

**Evidence:**
```go
case ipc.OpShutdown:
    json.NewEncoder(conn).Encode(ipc.Response{OK: true})
    s.Shutdown()  // no authentication — any same-user process can trigger this
    return
```

**Recommendation:** Issue a random 16-byte token at daemon startup, write it to a file with mode `0600` in `$XDG_RUNTIME_DIR`. Require the token in `OpShutdown` requests. This doesn't affect the attack surface for secrets (already protected by socket permissions) but prevents casual DoS.

---

### FINDING-004: Secret Value Exposed via Process Arguments (`set` command)

| Field | Value |
|---|---|
| Severity | Medium |
| Category | Secrets Handling |
| Location | `cmd/set.go:19` |
| CWE | CWE-214 (Sensitive Information in Process Environment / Arguments) |

**Description:** `secret-sauce set KEY VALUE` accepts the secret value as a command-line argument. On Linux, process arguments are readable by any user via `/proc/<pid>/cmdline` for the duration of the process lifetime, and are visible in `ps aux` output system-wide. They are also written to the shell's history (e.g., `~/.bash_history`, `~/.zsh_history`) unless the user explicitly suppresses it.

This is not new to this PR but it is the most broadly applicable issue in the codebase and has no existing documentation.

**Evidence:**
```go
// cmd/set.go:19
if err := svc.WriteSecret(vaultDir, args[0], args[1]); err != nil {
// args[1] is the raw secret value, which appeared in os.Args and /proc/<pid>/cmdline
```

**Recommendation:** Read the value from stdin when no argument is supplied, or always read from stdin (similar to `pass insert` behavior). Example: `secret-sauce set KEY` then prompts or reads from stdin. This keeps the secret out of the process argument list. Document this limitation clearly until it is addressed.

---

### FINDING-005: `waitForSocket` Uses File Existence, Not Liveness

| Field | Value |
|---|---|
| Severity | Low |
| Category | Access Control |
| Location | `cmd/service_resolver.go:70-78` |
| CWE | CWE-362 (TOCTOU) |

**Description:** After spawning the daemon, `waitForSocket` polls `os.Stat(socketPath)` to detect readiness. It returns `true` as soon as the socket *file* exists, which may be before the daemon's `os.Chmod(0600)` call completes (the same race from FINDING-001). `resolveService` then immediately returns an `IPCVaultService`, which will attempt to connect — and may succeed or fail depending on timing. In the worst case, a stale or attacker-created socket file at the path causes `resolveService` to return an IPC client that immediately errors, with no further fallback.

**Evidence:**
```go
func waitForSocket(socketPath string, timeout time.Duration) bool {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if _, err := os.Stat(socketPath); err == nil {
            return true  // file exists — but is the daemon actually listening?
        }
        time.Sleep(100 * time.Millisecond)
    }
    return false
}
```

**Recommendation:** Replace with a connection-and-ping attempt (same logic as `isSocketAlive`). This verifies the daemon is actually serving before `resolveService` commits to the IPC path.

---

### FINDING-006: `spawnDaemon` Uses `os.Args[0]` Instead of `os.Executable()`

| Field | Value |
|---|---|
| Severity | Informational |
| Category | Secrets Handling |
| Location | `cmd/service_resolver.go:62` |
| CWE | CWE-426 (Untrusted Search Path) |

**Description:** `os.Args[0]` contains whatever string the OS passed as the program name, which may be a relative path (e.g., `./secret-sauce`) or a symlink. If the current working directory has changed between startup and `spawnDaemon`, a relative path resolves differently. `os.Executable()` resolves the absolute path of the running binary and follows symlinks, making it unambiguous.

**Evidence:**
```go
c := exec.Command(os.Args[0], "daemon", "_serve", "--timeout", timeout.String())
```

**Recommendation:** Replace `os.Args[0]` with `os.Executable()`. Handle the error case by falling back to `LocalVaultService`.

---

## Accepted Risks (from THREAT_MODEL.md)

The following are acknowledged in the threat model and not re-raised:

- **Plaintext secret filenames** — documented as a current limitation; HMAC obfuscation is planned.
- **Unlocked session / D-Bus access** — explicitly accepted; mitigated at OS layer via session locking and Secret Service auto-lock.
- **Metadata leakage (file count/size analysis)** — explicitly accepted; chaffing rejected on performance grounds.
- **Recipient file integrity** — noted; mitigated by filesystem permissions and out-of-band key verification.

---

## Out of Scope

- Infrastructure (CI/CD pipelines, hosting)
- `rsync`/Git transport security (delegated to SSH/TLS)
- Secret Service provider implementations (KeePassXC, GNOME Keyring)
- OS-level memory scraping (`/proc/<pid>/mem`)
