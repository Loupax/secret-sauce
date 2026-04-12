<script>
  import { onMount } from 'svelte';
  import logo from './assets/logo.png';
  import { ListSecretNames, GetSecret, SetSecret, DeleteSecret, VaultExists, GetVaultDir } from '../wailsjs/go/main/App';
  import { ClipboardSetText } from '../wailsjs/runtime/runtime';

  // vault state
  let vaultReady = false;
  let vaultDir = '';
  let secretNames = [];
  let error = '';

  // search
  let query = '';
  $: trimmedQuery = query.trim();
  $: filteredNames = trimmedQuery
    ? secretNames.filter(n => n.toLowerCase().includes(trimmedQuery.toLowerCase()))
    : [];

  // expand / value cache
  let expanded = {};
  let secretData = {};
  let loading = {};
  let revealed = {};
  let copied = {};

  // "+" dropdown
  let showMenu = false;

  // create / edit secret modal
  let showCreate = false;
  let editingName = null; // null = create mode, string = edit mode
  let createName = '';
  let createFields = [{ key: 'value', value: '' }];
  let createError = '';
  let creating = false;

  // init vault dialog
  let showInitInfo = false;

  onMount(async () => {
    vaultDir = await GetVaultDir();
    vaultReady = await VaultExists();
    if (vaultReady) await refresh();
  });

  async function refresh() {
    try {
      secretNames = await ListSecretNames();
      secretData = {};
      expanded = {};
      error = '';
    } catch (e) {
      error = e.toString();
    }
  }

  async function toggle(name) {
    if (expanded[name]) {
      expanded[name] = false;
      expanded = expanded;
      return;
    }
    if (!secretData[name]) {
      loading[name] = true;
      loading = loading;
      try {
        const entry = await GetSecret(name);
        secretData[name] = entry.data ?? {};
      } catch (e) {
        error = e.toString();
        loading[name] = false;
        loading = loading;
        return;
      }
      loading[name] = false;
      loading = loading;
    }
    expanded[name] = true;
    expanded = expanded;
  }

  async function removeSecret(name) {
    try {
      await DeleteSecret(name);
      delete secretData[name];
      delete expanded[name];
      secretNames = secretNames.filter(n => n !== name);
    } catch (e) {
      error = e.toString();
    }
  }

  function toggleReveal(id) {
    revealed[id] = !revealed[id];
    revealed = revealed;
  }

  async function copyToClipboard(text, id) {
    try {
      await ClipboardSetText(text);
      copied[id] = true;
      copied = copied;
      setTimeout(() => { copied[id] = false; copied = copied; }, 1500);
    } catch (_) {}
  }

  // --- create secret ---

  function openCreate() {
    editingName = null;
    createName = '';
    createFields = [{ key: 'value', value: '' }];
    createError = '';
    showCreate = true;
    showMenu = false;
  }

  async function openEdit(name) {
    // Ensure data is fetched before opening modal.
    if (!secretData[name]) {
      loading[name] = true;
      loading = loading;
      try {
        const entry = await GetSecret(name);
        secretData[name] = entry.data ?? {};
      } catch (e) {
        error = e.toString();
        loading[name] = false;
        loading = loading;
        return;
      }
      loading[name] = false;
      loading = loading;
    }
    editingName = name;
    createName = name;
    createFields = Object.entries(secretData[name]).map(([key, value]) => ({ key, value }));
    if (createFields.length === 0) createFields = [{ key: 'value', value: '' }];
    createError = '';
    showCreate = true;
  }

  function closeCreate() {
    showCreate = false;
    editingName = null;
  }

  function addField() {
    createFields = [...createFields, { key: '', value: '' }];
  }

  function removeField(i) {
    createFields = createFields.filter((_, idx) => idx !== i);
  }

  async function submitCreate() {
    if (!createName.trim()) { createError = 'Name is required.'; return; }
    const data = {};
    for (const f of createFields) {
      if (!f.key.trim()) { createError = 'All field keys are required.'; return; }
      data[f.key.trim()] = f.value;
    }
    if (Object.keys(data).length === 0) { createError = 'Add at least one field.'; return; }
    creating = true;
    createError = '';
    try {
      await SetSecret(createName.trim(), data);
      // Clear cache so next expand re-fetches updated values.
      delete secretData[createName.trim()];
      secretData = secretData;
      if (editingName) {
        // Name list unchanged on edit; just close.
        closeCreate();
      } else {
        await refresh();
        closeCreate();
      }
    } catch (e) {
      createError = e.toString();
    } finally {
      creating = false;
    }
  }
</script>

<!-- backdrop to close the "+" dropdown -->
{#if showMenu}
  <div class="menu-backdrop" on:click={() => showMenu = false} role="presentation"></div>
{/if}

{#if !vaultReady}
  <div class="init-screen">
    <img src={logo} alt="Secret Sauce" class="logo" />
    <h2>No vault found</h2>
    <p>Run <code>sauce init</code> to initialize your vault, then restart the GUI.</p>
  </div>
{:else}
  <div class="layout">

    <!-- ── top bar ───────────────────────────────────────── -->
    <header>
      <img src={logo} alt="Secret Sauce" class="header-logo" />
      <h1>Secret Sauce</h1>

      <input
        class="search"
        bind:value={query}
        placeholder="Search secrets…"
        autocomplete="off"
        spellcheck="false"
      />

      <!-- "+" action menu -->
      <div class="menu-wrap">
        <button class="add-btn" on:click={() => showMenu = !showMenu} title="Actions">+</button>
        {#if showMenu}
          <div class="dropdown">
            <button class="dropdown-item" on:click={openCreate}>New Secret</button>
            <hr class="dropdown-sep" />
            <button class="dropdown-item muted" on:click={() => { showInitInfo = true; showMenu = false; }}>
              Init Vault
            </button>
          </div>
        {/if}
      </div>

      <button class="refresh-btn" on:click={refresh} title="Refresh">&#x21bb;</button>
    </header>

    <!-- error banner -->
    {#if error}
      <div class="error-banner">
        <span>{error}</span>
        <button class="retry-btn" on:click={() => { error = ''; refresh(); }}>Retry</button>
      </div>
    {/if}

    <!-- ── main content ──────────────────────────────────── -->
    <main>
      {#if trimmedQuery}
        <!-- search results -->
        <section class="results">
          {#if filteredNames.length === 0}
            <p class="no-match">No secrets match "<em>{trimmedQuery}</em>".</p>
          {:else}
            {#each filteredNames as name (name)}
              <div class="secret-card" class:expanded={expanded[name]}>
                <div class="secret-header"
                     on:click={() => toggle(name)}
                     role="button" tabindex="0"
                     on:keydown={e => e.key === 'Enter' && toggle(name)}>
                  <span class="secret-name">{name}</span>
                  <span class="chevron">{expanded[name] ? '▲' : '▼'}</span>
                  <button class="edit" on:click|stopPropagation={() => openEdit(name)}>Edit</button>
                  <button class="delete" on:click|stopPropagation={() => removeSecret(name)}>Delete</button>
                </div>

                {#if loading[name]}
                  <div class="loading">decrypting…</div>
                {:else if expanded[name] && secretData[name]}
                  <div class="fields">
                    {#each Object.entries(secretData[name]) as [k, v] (k)}
                      {@const rowId = name + '/' + k}
                      <div class="field-row">
                        <span class="field-key">{k}</span>
                        <input
                          type={revealed[rowId] ? 'text' : 'password'}
                          value={v}
                          readonly
                          class="field-value"
                        />
                        <button class="icon-btn" on:click={() => toggleReveal(rowId)}>
                          {revealed[rowId] ? 'Hide' : 'Show'}
                        </button>
                        <button class="icon-btn" on:click={() => copyToClipboard(v, rowId)}>
                          {copied[rowId] ? 'Copied!' : 'Copy'}
                        </button>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/each}
          {/if}
        </section>

      {:else}
        <!-- empty / welcome state -->
        <div class="welcome">
          <img src={logo} alt="" class="welcome-logo" />
          <p class="tagline">Search for a secret above, or press <kbd>+</kbd> to create one.</p>

          <div class="docs-grid">
            <div class="doc-card">
              <h3>Get a secret</h3>
              <pre>sauce get &lt;name&gt; &lt;field&gt;</pre>
              <p>Print a single field value to stdout.</p>
            </div>
            <div class="doc-card">
              <h3>Set a secret</h3>
              <pre>sauce set &lt;name&gt; &lt;field&gt;=&lt;value&gt;</pre>
              <p>Write or update a field. Creates the secret if it doesn't exist.</p>
            </div>
            <div class="doc-card">
              <h3>List secrets</h3>
              <pre>sauce ls</pre>
              <p>List all secret names in the vault.</p>
            </div>
            <div class="doc-card">
              <h3>Delete a secret</h3>
              <pre>sauce delete &lt;name&gt;</pre>
              <p>Permanently remove a secret and all its fields.</p>
            </div>
            <div class="doc-card">
              <h3>Export as env vars</h3>
              <pre>sauce export &lt;name&gt;</pre>
              <p>Print all fields as <code>KEY=VALUE</code> lines, ready to source.</p>
            </div>
            <div class="doc-card">
              <h3>Background daemon</h3>
              <pre>sauce daemon start</pre>
              <p>Start the daemon for fast repeated access without re-prompting the keychain.</p>
            </div>
          </div>
        </div>
      {/if}
    </main>

    <!-- ── status bar ────────────────────────────────────── -->
    <footer class="status-bar">
      <span class="vault-path" title="Vault directory">{vaultDir || '(resolving…)'}</span>
      <span class="secret-count">{secretNames.length} secret{secretNames.length === 1 ? '' : 's'}</span>
    </footer>

  </div>

  <!-- ── create secret modal ───────────────────────────────── -->
  {#if showCreate}
    <div class="modal-backdrop" on:click|self={closeCreate} on:keydown={e => e.key === 'Escape' && closeCreate()} role="dialog" aria-modal="true">
      <div class="modal">
        <div class="modal-header">
          <h2>{editingName ? 'Edit Secret' : 'New Secret'}</h2>
          <button class="close-btn" on:click={closeCreate}>✕</button>
        </div>

        <label class="field-label">
          Name
          <input class="modal-input" bind:value={createName} placeholder="e.g. github" autocomplete="off"
                 readonly={!!editingName} class:readonly-input={!!editingName} />
        </label>

        <div class="fields-section">
          <div class="fields-header">
            <span>Fields</span>
            <button class="add-field-btn" on:click={addField}>+ Add field</button>
          </div>
          {#each createFields as field, i (i)}
            <div class="create-field-row">
              <input
                class="modal-input key-input"
                bind:value={field.key}
                placeholder="key"
                autocomplete="off"
              />
              <input
                class="modal-input val-input"
                type="password"
                bind:value={field.value}
                placeholder="value"
                autocomplete="new-password"
              />
              {#if createFields.length > 1}
                <button class="remove-field-btn" on:click={() => removeField(i)}>✕</button>
              {/if}
            </div>
          {/each}
        </div>

        {#if createError}
          <p class="modal-error">{createError}</p>
        {/if}

        <div class="modal-actions">
          <button class="cancel-btn" on:click={closeCreate}>Cancel</button>
          <button class="submit-btn" on:click={submitCreate} disabled={creating}>
            {creating ? 'Saving…' : editingName ? 'Save' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- ── init vault info dialog ────────────────────────────── -->
  {#if showInitInfo}
    <div class="modal-backdrop" on:click|self={() => showInitInfo = false} on:keydown={e => e.key === 'Escape' && (showInitInfo = false)} role="dialog" aria-modal="true">
      <div class="modal">
        <div class="modal-header">
          <h2>Initialize a Vault</h2>
          <button class="close-btn" on:click={() => showInitInfo = false}>✕</button>
        </div>
        <p>Run in your terminal to create a new vault:</p>
        <pre class="code-block">sauce init</pre>
        <p>To use a custom location:</p>
        <pre class="code-block">SAUCE_DIR=/path/to/vault sauce init</pre>
        <p>Then restart the GUI.</p>
        <div class="modal-actions">
          <button class="submit-btn" on:click={() => showInitInfo = false}>Got it</button>
        </div>
      </div>
    </div>
  {/if}
{/if}

<style>
  :global(body) {
    margin: 0;
    background: #121212;
    color: #f0f0f0;
    font-family: system-ui, sans-serif;
    height: 100vh;
    overflow: hidden;
  }

  /* ── init screen ─────────────────────────────────────────── */
  .init-screen {
    text-align: center;
    margin-top: 6rem;
    padding: 2rem;
  }

  .logo { width: 120px; margin-bottom: 1rem; }
  .init-screen h2 { color: #e85d04; margin-bottom: 0.5rem; }

  code {
    background: #1e1e1e;
    padding: 0.15rem 0.45rem;
    border-radius: 3px;
    font-family: monospace;
    font-size: 0.9em;
  }

  /* ── main layout ─────────────────────────────────────────── */
  .layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  /* ── header ──────────────────────────────────────────────── */
  header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1.25rem;
    border-bottom: 1px solid #2a2a2a;
    flex-shrink: 0;
  }

  .header-logo { width: 28px; height: 28px; object-fit: contain; flex-shrink: 0; }

  header h1 {
    margin: 0;
    font-size: 1.1rem;
    color: #e85d04;
    letter-spacing: 0.02em;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .search {
    flex: 1;
    background: #1a1a1a;
    border: 1px solid #333;
    color: #f0f0f0;
    padding: 0.4rem 0.85rem;
    border-radius: 6px;
    font-size: 0.92rem;
    min-width: 0;
  }

  .search:focus { outline: none; border-color: #e85d04; }
  .search::placeholder { color: #555; }

  /* "+" menu */
  .menu-wrap { position: relative; flex-shrink: 0; }

  .add-btn {
    background: #e85d04;
    border: none;
    color: white;
    width: 32px;
    height: 32px;
    border-radius: 6px;
    font-size: 1.3rem;
    line-height: 1;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.15s;
  }

  .add-btn:hover { background: #ff6a1a; }

  .dropdown {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    background: #1e1e1e;
    border: 1px solid #333;
    border-radius: 8px;
    min-width: 160px;
    box-shadow: 0 8px 24px rgba(0,0,0,0.5);
    z-index: 100;
    overflow: hidden;
    padding: 0.25rem 0;
  }

  .dropdown-item {
    display: block;
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    color: #f0f0f0;
    padding: 0.55rem 1rem;
    font-size: 0.88rem;
    cursor: pointer;
    transition: background 0.1s;
    border-radius: 0;
  }

  .dropdown-item:hover { background: #2a2a2a; color: #fff; }
  .dropdown-item.muted { color: #888; }
  .dropdown-item.muted:hover { color: #ccc; }

  .dropdown-sep { border: none; border-top: 1px solid #2a2a2a; margin: 0.25rem 0; }

  .menu-backdrop {
    position: fixed;
    inset: 0;
    z-index: 99;
  }

  .refresh-btn {
    background: #2a2a2a;
    border: none;
    color: #ccc;
    font-size: 1.1rem;
    width: 32px;
    height: 32px;
    border-radius: 6px;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    transition: background 0.15s;
  }

  .refresh-btn:hover { background: #3a3a3a; color: #fff; }

  /* ── error banner ────────────────────────────────────────── */
  .error-banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    background: #2d0a0a;
    border-bottom: 1px solid #c0392b;
    color: #ff6b6b;
    padding: 0.6rem 1.25rem;
    font-size: 0.88rem;
    flex-shrink: 0;
  }

  .retry-btn {
    background: #c0392b;
    border: none;
    color: white;
    padding: 0.25rem 0.7rem;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.82rem;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .retry-btn:hover { background: #e74c3c; }

  /* ── main scrollable area ────────────────────────────────── */
  main {
    flex: 1;
    overflow-y: auto;
    padding: 1.25rem 1.5rem;
  }

  /* ── search results ──────────────────────────────────────── */
  .results {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }

  .no-match { color: #555; font-style: italic; font-size: 0.9rem; }

  .secret-card {
    background: #1a1a1a;
    border: 1px solid #2a2a2a;
    border-radius: 8px;
    overflow: hidden;
    transition: border-color 0.15s;
  }

  .secret-card.expanded { border-color: #3a3a3a; }

  .secret-header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.6rem 1rem;
    cursor: pointer;
    user-select: none;
  }

  .secret-header:hover { background: #222; }

  .secret-name {
    font-weight: 600;
    color: #e85d04;
    font-size: 0.92rem;
    flex: 1;
  }

  .chevron { color: #555; font-size: 0.65rem; }

  .loading {
    padding: 0.5rem 1rem;
    color: #555;
    font-style: italic;
    font-size: 0.83rem;
  }

  .fields {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    padding: 0.5rem 1rem 0.75rem;
    border-top: 1px solid #2a2a2a;
  }

  .field-row { display: flex; align-items: center; gap: 0.5rem; }

  .field-key {
    min-width: 80px;
    font-size: 0.76rem;
    color: #777;
    text-transform: lowercase;
    letter-spacing: 0.03em;
  }

  .field-value {
    flex: 1;
    background: #0e0e0e;
    border: 1px solid #333;
    color: #f0f0f0;
    padding: 0.28rem 0.6rem;
    border-radius: 4px;
    font-family: monospace;
    font-size: 0.88rem;
  }

  button {
    background: #e85d04;
    border: none;
    color: white;
    padding: 0.28rem 0.75rem;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.83rem;
    transition: background 0.15s;
  }

  button:hover { background: #ff6a1a; }
  button.edit { background: #1a3a5a; }
  button.edit:hover { background: #1f5080; }
  button.delete { background: #7a1a1a; }
  button.delete:hover { background: #c0392b; }

  .icon-btn {
    background: #2a2a2a;
    min-width: 58px;
    padding: 0.28rem 0.55rem;
    font-size: 0.76rem;
  }

  .icon-btn:hover { background: #383838; }

  /* ── welcome / empty state ───────────────────────────────── */
  .welcome {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding-top: 2.5rem;
  }

  .welcome-logo {
    width: 64px;
    opacity: 0.35;
    margin-bottom: 1rem;
  }

  .tagline {
    color: #555;
    font-size: 0.9rem;
    margin: 0 0 2.5rem;
    text-align: center;
  }

  kbd {
    background: #2a2a2a;
    border: 1px solid #444;
    border-radius: 3px;
    padding: 0.1rem 0.4rem;
    font-size: 0.85em;
    font-family: monospace;
    color: #ccc;
  }

  .docs-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.9rem;
    width: 100%;
    max-width: 700px;
  }

  .doc-card {
    background: #161616;
    border: 1px solid #252525;
    border-radius: 8px;
    padding: 1rem 1.1rem;
  }

  .doc-card h3 {
    margin: 0 0 0.45rem;
    font-size: 0.82rem;
    color: #888;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .doc-card pre {
    margin: 0 0 0.5rem;
    background: #0e0e0e;
    border: 1px solid #222;
    border-radius: 4px;
    padding: 0.4rem 0.65rem;
    font-size: 0.82rem;
    color: #e85d04;
    font-family: monospace;
    overflow-x: auto;
    white-space: pre;
  }

  .doc-card p {
    margin: 0;
    font-size: 0.8rem;
    color: #555;
    line-height: 1.5;
  }

  /* ── status bar ──────────────────────────────────────────── */
  .status-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.35rem 1.25rem;
    border-top: 1px solid #1e1e1e;
    background: #0e0e0e;
    flex-shrink: 0;
  }

  .vault-path {
    font-size: 0.68rem;
    color: #3a3a3a;
    font-family: monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .secret-count {
    font-size: 0.68rem;
    color: #3a3a3a;
    white-space: nowrap;
    flex-shrink: 0;
    margin-left: 1rem;
  }

  /* ── modals ──────────────────────────────────────────────── */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.65);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 200;
    padding: 1rem;
  }

  .modal {
    background: #1a1a1a;
    border: 1px solid #333;
    border-radius: 10px;
    padding: 1.5rem;
    width: 100%;
    max-width: 460px;
    box-shadow: 0 16px 48px rgba(0,0,0,0.6);
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1.25rem;
  }

  .modal-header h2 {
    margin: 0;
    font-size: 1.05rem;
    color: #f0f0f0;
    font-weight: 600;
  }

  .close-btn {
    background: none;
    border: none;
    color: #666;
    font-size: 1rem;
    cursor: pointer;
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
  }

  .close-btn:hover { background: #2a2a2a; color: #ccc; }

  .field-label {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    font-size: 0.8rem;
    color: #777;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 1.1rem;
  }

  .modal-input {
    background: #111;
    border: 1px solid #333;
    color: #f0f0f0;
    padding: 0.45rem 0.75rem;
    border-radius: 5px;
    font-size: 0.9rem;
    width: 100%;
    box-sizing: border-box;
  }

  .modal-input:focus { outline: none; border-color: #e85d04; }
  .readonly-input { color: #666; cursor: default; }
  .readonly-input:focus { border-color: #333; }

  .fields-section { margin-bottom: 1rem; }

  .fields-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.8rem;
    color: #777;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 0.5rem;
  }

  .add-field-btn {
    background: none;
    border: 1px solid #333;
    color: #888;
    font-size: 0.78rem;
    padding: 0.2rem 0.55rem;
    border-radius: 4px;
    cursor: pointer;
  }

  .add-field-btn:hover { background: #2a2a2a; color: #ccc; border-color: #444; }

  .create-field-row {
    display: flex;
    gap: 0.4rem;
    align-items: center;
    margin-bottom: 0.4rem;
  }

  .key-input { flex: 0 0 38%; }
  .val-input { flex: 1; }

  .remove-field-btn {
    background: none;
    border: none;
    color: #555;
    font-size: 0.85rem;
    padding: 0.2rem 0.35rem;
    cursor: pointer;
    border-radius: 3px;
    flex-shrink: 0;
  }

  .remove-field-btn:hover { background: #3a1a1a; color: #ff6b6b; }

  .modal-error {
    font-size: 0.82rem;
    color: #ff6b6b;
    margin: 0.5rem 0 0.75rem;
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.6rem;
    margin-top: 1.25rem;
  }

  .cancel-btn {
    background: #2a2a2a;
    color: #aaa;
  }

  .cancel-btn:hover { background: #333; color: #fff; }

  .submit-btn { background: #e85d04; }
  .submit-btn:hover { background: #ff6a1a; }
  .submit-btn:disabled { background: #5a3010; color: #888; cursor: default; }

  .code-block {
    background: #0e0e0e;
    border: 1px solid #2a2a2a;
    border-radius: 5px;
    padding: 0.6rem 0.9rem;
    font-size: 0.85rem;
    font-family: monospace;
    color: #e85d04;
    margin: 0.5rem 0 1rem;
    overflow-x: auto;
  }

  .modal p {
    font-size: 0.87rem;
    color: #888;
    margin: 0 0 0.25rem;
    line-height: 1.5;
  }
</style>
