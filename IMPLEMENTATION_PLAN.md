# Implementation Plan: Secret Types (`environment` / `file`)

> **For the Developer Agent.** Read every section before writing any code.
> Do not skip phases — each phase's output is consumed by the next.

---

## Phase 1 — Domain & Protocol Updates

### 1.1 `internal/vault/envelope.go`

**Goal:** Replace the single secret type with two well-named constants and remove
the now-wrong `"env_var"` string from the domain.

Changes required:

1. Delete the existing `SecretTypeEnvVar SecretType = "env_var"` constant.
2. Add two new constants:
   - `SecretTypeEnvironment SecretType = "environment"`
   - `SecretTypeFile        SecretType = "file"`
3. Add a validation helper `ValidSecretType(t SecretType) bool` that returns
   `true` only when `t` is one of the two constants above. This will be called
   from the CLI layer (Phase 3) and the service layer (Phase 2) to avoid
   scattered string comparisons.
4. `SecretEnvelope` already has a `Type SecretType` field — no struct changes
   needed, but confirm the JSON tag reads `"type"`.

Edge cases:
- Any file in the repository that references `SecretTypeEnvVar` must be updated
  to `SecretTypeEnvironment`. Run a codebase-wide search for `SecretTypeEnvVar`
  and `"env_var"` before finishing this phase. The daemon index and any test
  fixtures are the most likely locations.

---

### 1.2 `internal/vault/vault.go`

**Goal:** Propagate the `SecretType` into write operations and expose it on read
operations.

Changes required:

1. **`WriteSecret` signature change:**
   Current: `WriteSecret(vaultDir, key, value string, recipients []age.Recipient, identity age.Identity) error`
   New:     `WriteSecret(vaultDir, key, value string, secretType vault.SecretType, recipients []age.Recipient, identity age.Identity) error`
   The `secretType` must be stored in the `SecretEnvelope.Type` field before
   encrypting. Do not default silently — the caller must always pass a type.

2. **`ReadAllSecrets` return type change:**
   Current: `(map[string]string, error)`
   New:     `(map[string]SecretInfo, error)`
   where `SecretInfo` is a new struct (defined in this file or in a new
   `internal/vault/types.go`) with exactly two exported fields:
   ```
   type SecretInfo struct {
       Type  SecretType
       Value string
   }
   ```
   Populate `SecretInfo.Type` from `SecretEnvelope.Type` when decrypting.
   Populate `SecretInfo.Value` from `SecretEnvelope.Value`.

3. **`ReadSecret` return type change:**
   Current: `(string, error)`
   New:     `(SecretInfo, error)`
   Same population logic as above.

4. All existing internal callers inside `vault.go` itself (e.g., anything
   reading and then re-writing during `share add`) must be updated to the new
   signatures.

Edge cases:
- Files written by a previous version of the CLI will have `Type: "env_var"` in
  their envelope. During `ReadAllSecrets` / `ReadSecret`, if `envelope.Type` is
  `"env_var"` treat it as `SecretTypeEnvironment` transparently (a one-line
  migration shim). Do not silently discard or error — emit the correct type
  back to the caller.

---

### 1.3 `internal/ipc/ipc.go`

**Goal:** Extend the wire protocol to carry type information in both directions.

Changes required:

1. **`Request` struct:** Add `Type string \`json:"type,omitempty"\`` field. This
   will be set by `OpWrite` callers. It is ignored by `OpRead`, `OpDelete`,
   `OpPing`, and `OpShutdown` — the `omitempty` tag handles serialisation.

2. **`Response` struct:** Change `Secrets map[string]string` to
   `Secrets map[string]SecretMeta \`json:"secrets,omitempty"\`` where
   `SecretMeta` is a new struct in the `ipc` package:
   ```
   type SecretMeta struct {
       Type  string `json:"type"`
       Value string `json:"value"`
   }
   ```
   Keep `SecretMeta` in `ipc.go` (not imported from `vault`) to avoid a circular
   dependency. The service layer will perform the translation.

3. Update `SocketPath()` — no changes needed to its logic, but verify it is
   still referenced correctly after the struct changes.

---

## Phase 2 — Service Layer & Daemon Updates

### 2.1 `internal/service/service.go`

**Goal:** Update the `VaultService` interface to reflect the new payload shapes.

Changes required:

1. Change `ReadAllSecrets(vaultDir string) (map[string]string, error)` to
   `ReadAllSecrets(vaultDir string) (map[string]vault.SecretInfo, error)`.
2. Change `WriteSecret(vaultDir, key, value string) error` to
   `WriteSecret(vaultDir, key, value string, secretType vault.SecretType) error`.
3. `DeleteSecret` signature is unchanged.

Both concrete implementations (Local and IPC) must satisfy the updated
interface — the compiler will flag any gap.

---

### 2.2 `internal/service/local.go`

**Goal:** Implement the updated interface against the vault package.

Changes required:

1. `ReadAllSecrets`: Call `vault.ReadAllSecrets(...)` which now returns
   `map[string]vault.SecretInfo`. Return it directly — no translation needed.

2. `WriteSecret`: Accept the new `secretType vault.SecretType` parameter.
   Pass it through to `vault.WriteSecret(...)`.

3. No changes to `DeleteSecret`.

4. `loadIdentity` helper — no changes needed.

---

### 2.3 `internal/service/ipc_client.go`

**Goal:** Translate between the updated `VaultService` interface and the IPC wire format.

Changes required:

1. `ReadAllSecrets`: The `ipc.Response.Secrets` is now `map[string]ipc.SecretMeta`.
   Translate each entry into `vault.SecretInfo` before returning. Cast
   `meta.Type` (string) to `vault.SecretType`. Apply the same legacy shim as
   in Phase 1.2: if `meta.Type == "env_var"` treat as `SecretTypeEnvironment`.

2. `WriteSecret`: Accept `secretType vault.SecretType`. Set `req.Type =
   string(secretType)` on the `ipc.Request` before sending.

3. No changes to `DeleteSecret` or `dial` / `roundTrip`.

---

### 2.4 `internal/daemon/server.go`

**Goal:** Handle `Type` on incoming write requests and emit the new `SecretMeta`
map on `OpReadAll`.

Changes required:

1. `OpWrite` handler: Read `req.Type` from the incoming request. Validate it
   with `vault.ValidSecretType(vault.SecretType(req.Type))` — if invalid, return
   `ipc.Response{OK: false, Error: "invalid secret type"}` without writing.
   Pass `vault.SecretType(req.Type)` to `s.svc.WriteSecret(...)`.

2. `OpReadAll` handler: `s.svc.ReadAllSecrets(...)` now returns
   `map[string]vault.SecretInfo`. Build the `ipc.Response.Secrets` map of type
   `map[string]ipc.SecretMeta` by iterating and translating each entry:
   `ipc.SecretMeta{Type: string(info.Type), Value: info.Value}`.

3. The `VaultIndex` in `internal/daemon/index.go` stores `SecretEnvelope` which
   already has `Type`. Verify `refreshIndexIfStale` and the index's read path
   still populate `SecretEnvelope.Type` correctly after the constant rename.

---

## Phase 3 — Core CLI & Lifecycle

### 3.1 `cmd/set.go`

**Goal:** Require an explicit type as the first positional argument.

Changes required:

1. Change `Use` to `"set <type> <key> <value>"`.
2. Change `Args` to `cobra.ExactArgs(3)`.
3. In `RunE`:
   - `secretType := vault.SecretType(args[0])`
   - Validate with `vault.ValidSecretType(secretType)`. If invalid, return a
     user-friendly error: `"type must be 'environment' or 'file'; got %q"`.
   - `key := args[1]`, `value := args[2]`
   - Pass `secretType` to `svc.WriteSecret(vaultDir, key, value, secretType)`.

---

### 3.2 `cmd/run.go`

**Goal:** Only inject `environment`-typed secrets into the subprocess environment.

Changes required:

1. `svc.ReadAllSecrets(vaultDir)` now returns `map[string]vault.SecretInfo`.
2. When building `env`, iterate the map and skip any entry where
   `info.Type != vault.SecretTypeEnvironment`.
3. The rest of the subprocess-exec logic is unchanged.

---

### 3.3 `cmd/edit.go` (NEW FILE)

**Goal:** Open a decrypted secret in `$EDITOR`, wait for the editor to exit, then
re-encrypt and persist the updated value.

Create the file from scratch. Required elements:

1. **Command definition:**
   - `Use: "edit <type> <key>"`
   - `Short: "Edit a secret in $EDITOR"`
   - `Args: cobra.ExactArgs(2)`

2. **`RunE` implementation — exact sequence:**

   a. Parse and validate `secretType` from `args[0]` using
      `vault.ValidSecretType`. Return a clear error if invalid.

   b. `key := args[1]`

   c. Resolve the service: `svc, err := resolveService(vaultDir)`.

   d. Read the current value:
      `current, err := svc.ReadSecret(vaultDir, key)`
      Note: `ReadSecret` must be added to the `VaultService` interface (see
      below). If the key is not found, start with an empty string (do not error).

   e. Create a temp file:
      `tmp, err := os.CreateTemp("", "secret-sauce-edit-*")`
      Immediately after creation (before any other logic), register cleanup:
      `defer os.Remove(tmp.Name())`
      Set permissions: `tmp.Chmod(0600)` (or `os.Chmod(tmp.Name(), 0600)`).

   f. Write the current value to the temp file. Close it before launching the
      editor (most editors require the file handle to be free).

   g. Determine editor binary:
      `editor := os.Getenv("EDITOR")`
      If empty, try `"vi"`, then `"nano"` — use `exec.LookPath` to confirm
      availability. If none found, return an error with a clear message.

   h. Build and run the command:
      `cmd := exec.Command(editor, tmp.Name())`
      Bind `cmd.Stdin = os.Stdin`, `cmd.Stdout = os.Stdout`,
      `cmd.Stderr = os.Stderr`.
      Call `cmd.Run()`. If it returns a non-nil error, return it wrapped.

   i. Re-read the temp file contents after the editor exits.

   j. Persist: `svc.WriteSecret(vaultDir, key, string(contents), secretType)`.

3. **`ReadSecret` addition to `VaultService` interface:**
   Add `ReadSecret(vaultDir, key string) (vault.SecretInfo, error)` to
   `service/service.go`. Implement it in `local.go` (call `vault.ReadSecret`)
   and in `ipc_client.go` (new IPC round-trip using the existing `OpRead`
   op constant, or add `OpReadOne = "read_one"` to `ipc.go` if no single-key
   read op currently exists — check `ipc.go` first).

4. **Register the command** in `cmd/root.go` `init()`: add
   `rootCmd.AddCommand(editCmd)`.

Edge cases:
- If the user saves the file unchanged, still write (idempotent is correct here).
- If the editor exits with a non-zero code, do NOT persist. The `defer
  os.Remove` will still clean up.
- Do not leak the temp file path into the process environment.
- Trapping panics: use a named return + `recover()` in `RunE` only if a panic
  could leave the temp file behind beyond what `defer os.Remove` already handles
  — in practice the defer is sufficient; do not over-engineer this.

---

## Phase 4 — UX Enhancements

### 4.1 `cmd/ls.go`

**Goal:** Print tab-separated `<type>\t<key>` to enable UNIX pipeline composition.

Changes required:

1. `svc.ReadAllSecrets(vaultDir)` returns `map[string]vault.SecretInfo`.
2. Build a `[]string` of keys, sort alphabetically (existing logic).
3. Change the print loop:
   `fmt.Printf("%s\t%s\n", secrets[k].Type, k)`
4. Do not add headers or colours — raw tab-separated output only.

---

### 4.2 Shell Autocompletion via `ValidArgsFunction`

**Goal:** Wire Cobra's dynamic completion for `set`, `edit`, and `rm`.

General pattern: all completion functions must recover from errors by returning
`([]string{}, cobra.ShellCompDirectiveNoFileComp)` silently. Never write to
`os.Stderr` inside a `ValidArgsFunction` — it will corrupt the shell prompt.

**4.2.1 `cmd/set.go`**

Add `ValidArgsFunction` to `setCmd`:
```
ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    switch len(args) {
    case 0:
        return []string{"environment", "file"}, cobra.ShellCompDirectiveNoFileComp
    case 1:
        // return existing keys
        svc, err := resolveService(vaultDir)
        if err != nil { return nil, cobra.ShellCompDirectiveNoFileComp }
        secrets, err := svc.ReadAllSecrets(vaultDir)
        if err != nil { return nil, cobra.ShellCompDirectiveNoFileComp }
        keys := make([]string, 0, len(secrets))
        for k := range secrets { keys = append(keys, k) }
        return keys, cobra.ShellCompDirectiveNoFileComp
    default:
        return nil, cobra.ShellCompDirectiveNoFileComp
    }
}
```

**4.2.2 `cmd/edit.go`**

Add identical `ValidArgsFunction` to `editCmd` — same logic as `set` above,
since the argument positions are the same (`<type>` then `<key>`).

**4.2.3 `cmd/rm.go`**

Add `ValidArgsFunction` to `rmCmd`:
```
ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    if len(args) != 0 { return nil, cobra.ShellCompDirectiveNoFileComp }
    svc, err := resolveService(vaultDir)
    if err != nil { return nil, cobra.ShellCompDirectiveNoFileComp }
    secrets, err := svc.ReadAllSecrets(vaultDir)
    if err != nil { return nil, cobra.ShellCompDirectiveNoFileComp }
    keys := make([]string, 0, len(secrets))
    for k := range secrets { keys = append(keys, k) }
    return keys, cobra.ShellCompDirectiveNoFileComp
}
```

Note on `DisableFlagParsing` in `run.go`: Cobra's completion system does not
invoke `ValidArgsFunction` when `DisableFlagParsing` is true — this is expected
and no workaround is needed.

---

## Phase 5 — Documentation Synchronization

> Read the actual current content of `README.md` and `CHANGELOG.md` before
> editing. Do not document features that are not yet implemented.

### 5.1 `README.md`

1. Update the **Usage** section for `set`:
   - Show the new three-argument form: `secret-sauce set <type> <key> <value>`
   - Add a usage example for each type.
2. Add a **`edit`** command entry with the `$EDITOR` workflow described.
3. Update the **`ls`** output example to show the tab-separated format.
4. Add a **`run`** note clarifying that only `environment`-typed secrets are
   injected as environment variables.
5. Add a short **Secret Types** section explaining the difference between
   `environment` and `file`.

### 5.2 `CHANGELOG.md`

Add a new entry at the top (do not modify existing entries). Include:
- Breaking change: `set` now requires a type argument.
- Breaking change: `ls` output format changed to `<type>\t<key>`.
- New command: `edit`.
- New secret type: `file`.
- `run` now filters to `environment` secrets only.
- Shell autocompletion for `set`, `edit`, `rm`.

---

## Cross-Cutting Concerns

- **Test updates:** Every existing test in `internal/vault/vault_test.go` and
  `internal/service/` that calls `WriteSecret` or `ReadAllSecrets` will fail to
  compile after Phase 1–2. Update each call site to pass `SecretTypeEnvironment`
  as the default type. Do not delete tests.
- **`share add` re-encryption loop** (`cmd/share.go`): calls `vault.WriteSecret`
  internally — update to pass the envelope's existing `Type` through (read it
  from `SecretEnvelope.Type` during the re-encrypt walk).
- **`cmd/daemon.go` `_serve`**: passes through to `daemon.NewServer` — no
  changes needed unless `NewServer` signature changes (it should not).
- **Compilation gate:** After each phase, the codebase must compile cleanly with
  `go build ./...`. Do not move to the next phase until compilation passes.
