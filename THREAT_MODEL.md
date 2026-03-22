# Threat Model: secret-sauce

**Version:** pre-release
**Last updated:** 2026-03-22
**Scope:** local CLI vault, directory-on-disk storage, OS Secret Service integration, `rsync`/Git synchronization

---

## Overview

`secret-sauce` is a local-first, encrypted secret vault implemented as a CLI wrapper. Secrets are encrypted at rest using the `age` cryptographic library (X25519/ChaCha20-Poly1305) and injected as environment variables into child processes at execution time. The private key never touches disk; it is stored exclusively in the OS Secret Service (D-Bus).

This document describes the threat vectors this tool is explicitly designed to address, the risks it explicitly accepts, and the boundaries beyond which users must apply independent controls.

---

## Cryptographic Foundations

### Encryption Primitive

All secrets are encrypted with [`filippo.io/age`](https://age-encryption.org/), version 1. The specific cipher suite in use is:

- **Key agreement:** X25519 (Curve25519 ECDH)
- **Key encapsulation:** HKDF-SHA256 with a per-file random salt and a fixed context string
- **Payload encryption:** ChaCha20-Poly1305 (AEAD)

Each `.age` file contains a complete, self-describing header with one or more recipient stanzas, followed by the ciphertext payload. This means each secret file independently carries the key material needed to decrypt it for any authorized recipient, enabling multi-recipient vaults without a separate key distribution mechanism.

### Key Material Lifecycle

1. **Generation:** On `secret-sauce init`, a fresh X25519 identity is generated using the OS CSPRNG via `age.GenerateX25519Identity()`.
2. **Private key storage:** The private key (Bech32-encoded `AGE-SECRET-KEY-1…` string) is stored in the OS Secret Service via D-Bus (e.g., KeePassXC Secret Service, GNOME Keyring). It is **never written to disk by this tool**. The keyring lookup key is `SHA-256(vaultDir)`, namespaced under the service identifier `secret-sauce`.
3. **Public key storage:** The X25519 public key (recipient) is written to `.vault_recipients` in plaintext. This file is not sensitive — public keys are, by design, safe to publish.
4. **Runtime access:** On any command that requires decryption (`run`, `ls`, `share add`), the private key is retrieved from the Secret Service for the duration of the operation and held only in process memory.
5. **Multi-recipient re-encryption:** `share add` decrypts every secret concurrently and re-encrypts each one for the full updated recipient list, maintaining `O(1)` ciphertext per recipient rather than per-secret.

### Write Atomicity

`WriteSecret` uses a write-to-temp-file-then-rename pattern (`O_CREATE` temp + `fsync` + `rename(2)`). On Linux, `rename(2)` is atomic with respect to filesystem visibility. This prevents a reader from observing a partially-written ciphertext.

### Concurrency Safety

Read operations acquire a `LOCK_SH` `flock(2)` on `vault.lock`. Write operations (`set`, `rm`) and structural modifications (`share add`) acquire `LOCK_EX`. This prevents concurrent `run` invocations from racing with in-progress `set` operations.

---

## Threat Model

### Threat 1: Secret Confidentiality at Rest

**Threat:** An attacker gains read access to the vault directory (e.g., via a compromised backup, a stolen disk, a misconfigured file share, or a public Git repository containing the vault).

**Mitigation:** All secret values are encrypted with ChaCha20-Poly1305 under per-recipient X25519 key encapsulation. An attacker who possesses only the vault directory and not the corresponding private key cannot decrypt any secret value. The security reduces to the hardness of ECDH on Curve25519, which provides approximately 128 bits of security.

**Residual exposure:** The vault directory contains `.vault_recipients`, a plaintext list of X25519 public keys. Public keys are not sensitive. The `vault.lock` file is an empty coordination file and contains no secret material.

**Risk disposition:** Mitigated by design.

---

### Threat 2: Secret Name Leakage (Plaintext Filenames)

**Threat:** Secrets are stored as `<secret_name>.age` files with plaintext names. An attacker with read access to the vault directory — including any system, person, or service that can list the directory — learns the full set of secret names without requiring the private key. For example, the presence of `STRIPE_SECRET_KEY.age`, `SENDGRID_API_KEY.age`, and `DATABASE_URL.age` immediately fingerprints the technology stack and the nature of the stored credentials, providing targeted attack vectors.

**Current state:** The plaintext filename scheme is the current implementation. No filename obfuscation is present.

**Planned mitigation (not yet implemented):** A Zero-Knowledge filename architecture is planned. The design generates a high-entropy 32-byte HMAC key, encrypts it via `age` into a `vault_hmac_key.age` file, and uses it to derive deterministic HMAC-SHA256 filenames for each secret. Under this scheme, filenames become opaque 64-character hex strings. The HMAC key's encryption under the vault's recipient key ensures that filename reversal requires possession of the private key. The deterministic derivation (`HMAC-SHA256(hmac_key, secret_name)`) preserves `rsync` delta-transfer efficiency — the same secret always maps to the same filename, so unchanged secrets are not retransmitted.

**Current risk acceptance:** Until the HMAC filename architecture is implemented, users should treat the vault directory as leaking secret names to any party with filesystem read access. **Do not store a plaintext-filename vault in a public Git repository or on a shared filesystem if secret names are sensitive.**

**Risk disposition:** Partially accepted (current). Planned mitigation in development.

---

### Threat 3: Physical or Remote Access to an Unlocked Session (Context Hijacking)

**Threat:** If an attacker gains physical access to an active, unlocked desktop session, or achieves remote code execution as the user, they can:

1. **Query the D-Bus Secret Service directly** — tools such as `secret-tool`, `keyctl`, or a custom D-Bus client can retrieve the raw `AGE-SECRET-KEY-1…` private key without invoking `secret-sauce` at all.
2. **Observe process environment** — if `secret-sauce run` is executing, `/proc/<pid>/environ` exposes the injected secrets in plaintext for the duration of the child process's lifetime.
3. **Read from `/proc/<pid>/mem`** — with sufficient privilege, an attacker can dump process memory containing decrypted secret values.

**Risk acceptance:** This tool explicitly accepts this risk class. `secret-sauce` is designed to protect data **at rest** (encrypted files) and **in transit during synchronization** (ciphertext pushed via `rsync` or Git). It is not a defense against a compromised or unattended endpoint. This is a deliberate scope boundary, not a deficiency.

**Delegated mitigations:** Endpoint security is the responsibility of the operating system and user configuration:

- **Session locking:** Configure `swayidle`/`swaylock` (Sway), `xss-lock` (X11), or equivalent to lock the session after a short idle period. A locked session prevents physical access attacks.
- **Secret Service auto-lock:** Configure your Secret Service provider to require re-authentication after inactivity (KeePassXC: *Tools → Settings → Security → Lock database after inactivity*; GNOME Keyring locks automatically on screen lock).
- **Process environment isolation:** The `run` subcommand releases the vault lock and passes secrets only to the immediate child process's environment. Secrets are not written to any file or exported to the parent shell. The exposure window is bounded to the child process lifetime.

**Note on the "daemon" pattern:** This tool does not implement a background daemon or IPC socket. Keyring access is a direct, synchronous D-Bus call per invocation. There is no long-lived process holding decrypted key material between commands. Each invocation's private key access begins and ends within the command's execution lifetime, which limits the blast radius of process-level attacks.

**Risk disposition:** Explicitly accepted. Mitigated at the OS layer.

---

### Threat 4: Metadata Leakage via Traffic Analysis (File Count and Size)

**Threat:** When the vault directory is synchronized to a shared or public location (Git repository, cloud storage), an observer can analyze the vault's metadata without decrypting any content:

- **File count:** The number of `.age` files reveals the number of secrets.
- **File sizes:** `age` encrypted payloads have a deterministic size relationship to plaintext length (fixed-size header + 64-byte AEAD chunks). An attacker can estimate the character length of each secret value from the ciphertext size, potentially distinguishing short API tokens from long database connection strings or certificates.
- **Commit-correlated fingerprinting:** By correlating Git commit timestamps with changes in file count and size, an attacker can fingerprint integration events (e.g., "two new files of approximately 32 and 128 bytes appeared after this commit — they likely added a Stripe key pair").

**Risk acceptance:** This metadata risk is explicitly accepted. The following potential mitigations were evaluated and rejected:

- **Cryptographic chaffing (padding with fake files):** Adding randomized decoy `.age` files to obscure true file count would require maintaining, synchronizing, and auditing a chaff set. It would increase `rsync`/Git transfer sizes and introduce complexity with no cryptographic guarantee of indistinguishability.
- **Uniform payload padding:** Padding all secret values to a fixed size before encryption would cap the maximum secret length, waste storage, and complicate the implementation. It would destroy the sub-100ms concurrent decryption performance that the `run` subcommand depends on.

This tool prioritizes **rapid, transparent CLI execution** and **repository cleanliness** over absolute resistance to traffic analysis. Users who require metadata anonymity against an active, surveillance-capable adversary should not synchronize their vault to a public or adversary-observable location under any circumstances, regardless of this tool's protections.

**Risk disposition:** Explicitly accepted. Not mitigated.

---

### Threat 5: Recipient File Integrity (Trust on First Use)

**Threat:** The `.vault_recipients` file is stored in plaintext and is not integrity-protected by the vault's cryptographic scheme. An attacker with write access to the vault directory can append a recipient they control. The next `share add` or any command that re-encrypts secrets (currently only `share add`) would then encrypt secrets for the attacker's key.

**Current state:** There is no signature or MAC over `.vault_recipients`. The file is trusted implicitly on read.

**Mitigations:**

- Filesystem permissions: `Init` creates the vault directory with mode `0700` and the recipients file with mode `0600`. These permissions prevent other local users from writing to the file.
- Repository integrity: If the vault is tracked in a Git repository with signed commits, the Git object model provides tamper evidence for changes to `.vault_recipients`.
- Out-of-band verification: When adding a teammate's public key via `share add`, verify the key fingerprint through a separate trusted channel (e.g., in-person, Signal, or a signed keybase profile).

**Risk disposition:** Partially mitigated. Users sharing vaults in team environments should verify recipient key additions out of band.

---

### Threat 6: Keyring Availability and Reliability

**Threat:** `secret-sauce` depends on a D-Bus Secret Service provider being active at the time of any operation requiring the private key. If the Secret Service is unavailable (daemon not running, D-Bus session bus unreachable), all vault operations fail with an error and no fallback is available.

**Design intent:** This is a deliberate constraint. Storing the private key on disk — even encrypted — would require managing a passphrase or key-encryption key, reintroducing the problem the Secret Service solves. The D-Bus dependency is the price of not persisting the private key to disk.

**Practical guidance:**

- Ensure your Secret Service provider (`keepassxc`, `gnome-keyring-daemon`, etc.) is started before invoking `secret-sauce`.
- For headless or CI/CD environments, the Secret Service pattern is unsuitable. Consider a different key management strategy (e.g., `age` with a hardware key, or a dedicated secrets manager) for non-interactive contexts.

**Risk disposition:** Accepted by design. Operational dependency, not a security vulnerability.

---

## Out of Scope

The following threats are explicitly outside the scope of this tool and are not mitigated:

| Threat | Rationale |
|---|---|
| Kernel-level keyloggers or memory scrapers | Requires OS-level hardening (Secure Boot, kernel lockdown, SELinux/AppArmor) |
| Compromised `age` library supply chain | Upstream dependency; mitigated by Go module checksum verification (`go.sum`) |
| Brute-force of X25519 private key | Computationally infeasible at current security levels (~2^128 operations) |
| Secrets leaked via shell history | `run` does not pass secrets as shell arguments; they are injected as environment variables |
| Secrets leaked via child process logs | Application-level concern; out of scope for a vault tool |
| Network-layer interception of vault sync | Delegated to the transport (`rsync` over SSH, HTTPS Git remotes with certificate validation) |

---

## Security Contact

To report a vulnerability, open a confidential issue or contact the maintainer directly. Do not disclose security vulnerabilities in public issue tracker comments.
