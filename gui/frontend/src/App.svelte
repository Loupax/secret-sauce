<script>
  import { onMount } from 'svelte';
  import logo from './assets/logo.png';
  import { ListSecretNames, GetSecret, SetSecret, DeleteSecret, VaultExists, GetVaultDir } from '../wailsjs/go/main/App';
  import { ClipboardSetText } from '../wailsjs/runtime/runtime';

  let vaultReady = false;
  let secretNames = [];
  let expanded = {};   // name → bool
  let secretData = {}; // name → {field: value}, cached after first fetch
  let loading = {};    // name → bool
  let newKey = '';
  let newValue = '';
  let error = '';
  let copied = {};
  let vaultDir = '';

  onMount(async () => {
    vaultDir = await GetVaultDir();
    vaultReady = await VaultExists();
    if (vaultReady) await refresh();
  });

  async function refresh() {
    try {
      secretNames = await ListSecretNames();
      // Clear cached data so re-expand re-fetches fresh values.
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
    // Fetch data if not cached.
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

  async function addSecret() {
    if (!newKey || !newValue) return;
    try {
      await SetSecret(newKey, { value: newValue });
      newKey = '';
      newValue = '';
      await refresh();
    } catch (e) {
      error = e.toString();
    }
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

  let revealed = {};
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
</script>

{#if !vaultReady}
  <div class="init-screen">
    <img src={logo} alt="Secret Sauce" class="logo" />
    <h2>No vault found</h2>
    <p>Run <code>sauce init</code> to initialize your vault, then restart the GUI.</p>
  </div>
{:else}
  <div class="dashboard">
    <header>
      <img src={logo} alt="Secret Sauce" class="header-logo" />
      <h1>Secret Sauce</h1>
      <button class="refresh-btn" on:click={refresh} title="Refresh secrets">&#x21bb;</button>
    </header>
    <div class="vault-path" title="Vault directory">{vaultDir || '(resolving…)'}</div>

    {#if error}
      <div class="error">
        {error}
        <button class="retry-btn" on:click={refresh}>Retry</button>
      </div>
    {/if}

    <section class="secrets-list">
      {#if secretNames.length === 0}
        <p class="empty">No secrets stored yet.</p>
      {/if}
      {#each secretNames as name (name)}
        <div class="secret-card" class:expanded={expanded[name]}>
          <div class="secret-header" on:click={() => toggle(name)} role="button" tabindex="0"
               on:keydown={e => e.key === 'Enter' && toggle(name)}>
            <span class="secret-name">{name}</span>
            <span class="chevron">{expanded[name] ? '▲' : '▼'}</span>
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
    </section>

    <section class="add-form">
      <h3>Add Secret</h3>
      <div class="add-row">
        <input bind:value={newKey} placeholder="Secret name" class="add-input" />
        <input type="password" bind:value={newValue} placeholder="Value" class="add-input" />
        <button on:click={addSecret}>Add</button>
      </div>
    </section>
  </div>
{/if}

<style>
  :global(body) {
    margin: 0;
    background: #121212;
    color: #f0f0f0;
    font-family: system-ui, sans-serif;
  }

  .init-screen {
    text-align: center;
    margin-top: 6rem;
    padding: 2rem;
  }

  .logo { width: 120px; margin-bottom: 1rem; }

  .init-screen h2 { color: #e85d04; margin-bottom: 0.5rem; }

  code {
    background: #1e1e1e;
    padding: 0.2rem 0.5rem;
    border-radius: 3px;
    font-family: monospace;
  }

  .dashboard {
    padding: 1.5rem 2rem;
    max-width: 900px;
    margin: 0 auto;
  }

  header {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-bottom: 0.5rem;
    border-bottom: 1px solid #2a2a2a;
    padding-bottom: 1rem;
  }

  .header-logo { width: 36px; height: 36px; object-fit: contain; }

  header h1 {
    margin: 0;
    font-size: 1.4rem;
    color: #e85d04;
    letter-spacing: 0.02em;
  }

  .vault-path {
    font-size: 0.72rem;
    color: #555;
    font-family: monospace;
    margin-bottom: 1.5rem;
  }

  .refresh-btn {
    margin-left: auto;
    background: #2a2a2a;
    color: #ccc;
    font-size: 1.2rem;
    padding: 0.2rem 0.6rem;
    border-radius: 4px;
  }

  .refresh-btn:hover { background: #3a3a3a; color: #fff; }

  .error {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    background: #2d0a0a;
    border: 1px solid #c0392b;
    color: #ff6b6b;
    padding: 0.75rem 1rem;
    border-radius: 6px;
    margin-bottom: 1rem;
  }

  .retry-btn { background: #c0392b; white-space: nowrap; flex-shrink: 0; }
  .retry-btn:hover { background: #e74c3c; }

  .secrets-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    margin-bottom: 2rem;
  }

  .empty { color: #555; font-style: italic; }

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
    padding: 0.65rem 1rem;
    cursor: pointer;
    user-select: none;
  }

  .secret-header:hover { background: #222; }

  .secret-name {
    font-weight: 600;
    color: #e85d04;
    font-size: 0.95rem;
    flex: 1;
  }

  .chevron { color: #555; font-size: 0.7rem; }

  .loading {
    padding: 0.5rem 1rem;
    color: #555;
    font-style: italic;
    font-size: 0.85rem;
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
    font-size: 0.78rem;
    color: #777;
    text-transform: lowercase;
    letter-spacing: 0.03em;
  }

  .field-value {
    flex: 1;
    background: #0e0e0e;
    border: 1px solid #333;
    color: #f0f0f0;
    padding: 0.3rem 0.6rem;
    border-radius: 4px;
    font-family: monospace;
    font-size: 0.9rem;
  }

  button {
    background: #e85d04;
    border: none;
    color: white;
    padding: 0.3rem 0.8rem;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
    transition: background 0.15s;
  }

  button:hover { background: #ff6a1a; }
  button.delete { background: #7a1a1a; }
  button.delete:hover { background: #c0392b; }

  .icon-btn {
    background: #2a2a2a;
    min-width: 60px;
    padding: 0.3rem 0.6rem;
    font-size: 0.78rem;
  }

  .icon-btn:hover { background: #383838; }

  .add-form { border-top: 1px solid #2a2a2a; padding-top: 1.5rem; }

  .add-form h3 {
    margin: 0 0 0.75rem;
    color: #ccc;
    font-size: 1rem;
    font-weight: 500;
  }

  .add-row { display: flex; gap: 0.5rem; align-items: center; }

  .add-input {
    background: #1a1a1a;
    border: 1px solid #333;
    color: #f0f0f0;
    padding: 0.4rem 0.75rem;
    border-radius: 4px;
    font-size: 0.9rem;
    flex: 1;
  }

  .add-input:focus { outline: none; border-color: #e85d04; }
</style>
