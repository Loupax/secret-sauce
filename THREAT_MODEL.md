# Threat Model

This document describes the security assumptions, explicit risk acceptances, and
architectural trade-offs of `secret-sauce`. It is intended to help users make an
informed decision about whether this tool fits their threat model.

`secret-sauce` is designed to protect **secrets at rest** and **during synchronisation**.
It is not a defence against an attacker who has achieved local session access.

---

## Assets

| Asset | Where it lives | Protection mechanism |
|---|---|---|
| Secret values | `<KEY>.age` files on disk | `age` X25519 envelope encryption |
| Private key | OS Secret Service (D-Bus) / daemon memory | Never written to disk |
| Secret names | Filename of each `.age` file | **None — plaintext** (see §2) |
| Recipient public keys | `.vault_recipients` plaintext | None — public by design |

---

## 1. Physical Access & Unlocked Session (Context Hijacking)

### Threat

If an attacker gains physical or remote shell access to a user's **active, unlocked
session**, they can:

- Query the OS Secret Service over D-Bus to retrieve the stored private key directly.
- Connect to the daemon's Unix Domain Socket (`$XDG_RUNTIME_DIR/secret-sauce.sock`)
  and issue `read_all` requests to dump all plaintext secret values.
- Execute `secret-sauce run -- env` directly, as the tool runs with the user's
  credentials.

This is a **full compromise** of all vault secrets.

### Risk acceptance

This risk is **explicitly accepted**. `secret-sauce` is a local-first tool that
delegates session security entirely to the operating system. Defence against an attacker
with active session access would require hardware security modules, multi-party
computation, or an always-present remote secret service — all of which are outside the
scope and goals of this project.

### Mitigations in place

- **Idle timeout** — the daemon drops the private key from process memory and removes
  the IPC socket after a configurable idle period (default: 15 minutes). This
  significantly reduces the window during which the socket can be queried without
  re-authenticating against the keyring. An attacker who gains access to a session that
  has been idle longer than the timeout cannot use the socket; they must trigger a
  fresh keyring access, which may prompt the user.

- **Socket permissions** — the Unix Domain Socket is created with `0600` permissions,
  restricting connections to the owning user. Other local users cannot connect to the
  daemon.

- **Private key never touches disk** — the private key lives only in the OS Secret
  Service and transiently in daemon process memory. A snapshot of the filesystem (e.g.,
  a stolen disk, a cloud VM snapshot) does not expose the private key.

### Recommended OS-level controls

- Use a screen locker (`swaylock`, `i3lock`) with automatic activation via an idle
  daemon (`swayidle`, `xss-lock`) configured to a timeout shorter than the
  `secret-sauce` daemon idle timeout.
- Encrypt your home partition (LUKS or equivalent) to protect `.age` files at rest
  against physical disk theft.

---

## 2. Secret Name Leakage (Plaintext Filenames)

### Threat

`secret-sauce` stores each secret as a file named `<KEY>.age` — for example,
`STRIPE_SECRET_KEY.age` or `DATABASE_URL.age`. The secret name is **not encrypted**.

Any party with read access to the vault directory can enumerate all secret names without
possessing the private key. In practice this means:

- A **Git repository** hosting the vault leaks all secret names to anyone who can read
  the repository, including in historical commits.
- A **shared filesystem** or an `rsync` target where the receiving side should not know
  what secrets are present still exposes the key names.
- An attacker who can observe `ls` output or directory listings on a compromised system
  learns the exact name of every secret before attempting to retrieve the values.

Secret names are often high-signal: `STRIPE_SECRET_KEY`, `OPENAI_API_KEY`, and
`PROD_DB_PASSWORD` reveal the user's integrations, technology stack, and environment
topology — useful intelligence for a targeted attack even before any values are
decrypted.

### Risk acceptance

This risk is **explicitly accepted in the current implementation**. The vault is
designed for fast concurrent decryption (all `.age` files read in parallel) and
`rsync`/`git` compatibility (each secret is a separate, independently diffable file).
Opaque filenames would require an additional encrypted key-to-filename mapping, adding
latency and implementation complexity.

### Future mitigation path

A content-addressed filename scheme would eliminate this leakage without sacrificing
`rsync` compatibility:

1. Generate a high-entropy 32-byte HMAC key at vault initialisation time.
2. Encrypt that key to all vault recipients as `vault_hmac_key.age`.
3. Derive each secret's filename as `HMAC-SHA256(hmac_key, key_name)`, truncated and
   hex-encoded (e.g., `a3f8c1...age`).

This makes filenames deterministic (so `rsync` still detects renames correctly) but
computationally irreversible without the HMAC key, which is itself protected by the same
`age` envelope encryption as the secrets. Dictionary attacks against the filenames
become infeasible because the HMAC key provides 256 bits of entropy as the salt.

**This feature is not yet implemented.** Until it is, users who require secret name
confidentiality should not push their vault to a shared or public repository.

---

## 3. Traffic Analysis (File Count & Size Correlation)

### Threat

Even with encrypted values and opaque filenames (see §2), an attacker observing a vault
pushed to a **public Git repository** or an **externally visible storage location** can
perform traffic analysis:

- **File count** — the number of `.age` files reveals the number of secrets. Commit
  history shows when secrets were added or removed.
- **File sizes** — `age` adds a fixed overhead per recipient. The remaining payload size
  correlates with the length of the secret value. A 40-character API key and a
  300-character database connection string produce distinguishably different file sizes.
- **Timing correlation** — a commit adding two files shortly after a vendor's public
  onboarding flow can fingerprint integration choices (e.g., "they integrated payment
  processing and a monitoring service on the same day").

### Risk acceptance

This risk is **explicitly accepted**. Effective countermeasures would require:

- **Payload padding** — inflating all encrypted values to a uniform size, destroying the
  file size signal. This wastes storage and, more critically, increases decryption
  time proportionally to the padding factor for every `run` invocation.
- **Chaffing** — filling the vault with randomised fake `.age` files to obscure the
  true secret count. This adds noise to `rsync` transfers, inflates repository history,
  and provides only probabilistic — not cryptographic — obscuration.

Both measures conflict with the primary design goal of **sub-100 ms startup overhead**
for a CLI wrapper used in tight development loops (e.g., `secret-sauce run -- npm start`
executed dozens of times per day). Repository cleanliness and predictable performance
are prioritised over traffic analysis immunity.

### Recommended operational controls

- Do not push a vault containing sensitive secrets to a public repository.
- If using a private repository, evaluate your organisation's threat model against
  an insider with repository read access learning your secret topology.
- For high-confidentiality environments, use a private sync mechanism (`rsync` over SSH
  to a private target) rather than a hosted Git service.

---

## 4. Cryptographic Assumptions

The security of secret values at rest depends on the following holding:

| Assumption | Basis |
|---|---|
| X25519 key exchange is secure | Widely audited; ECDH over Curve25519 |
| ChaCha20-Poly1305 AEAD is secure | `age` payload cipher; IETF standard |
| The OS Secret Service does not leak the private key | Delegated to KeePassXC / GNOME Keyring / KWallet |
| The `age` library implementation is correct | [`filippo.io/age`](https://pkg.go.dev/filippo.io/age) — audited by Cure53 (2021) |

`secret-sauce` does not implement any cryptography itself. All cryptographic operations
are delegated to `filippo.io/age`.

---

## 5. Out of Scope

The following are explicitly outside the security goals of this tool:

- **Multi-user access control beyond encryption** — any holder of a valid private key
  can read all secrets. There is no role-based access control or per-secret ACL.
- **Audit logging** — no record is kept of which identity decrypted which secret or when.
- **Revocation** — removing a recipient requires re-encrypting every secret; there is
  no mechanism to retroactively revoke access to secrets they may have already decrypted
  and cached.
- **Forward secrecy** — if a private key is compromised, all past secrets encrypted to
  that key are also compromised. Key rotation requires re-initialising the vault.
- **Side-channel resistance** — no measures are taken against timing or cache-side-channel
  attacks on the decryption path.
- **Windows and macOS** — unsupported platforms with different keyring APIs and
  filesystem semantics.
