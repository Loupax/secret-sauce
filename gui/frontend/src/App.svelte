<script>
  import { onMount } from 'svelte';
  import logo from './assets/logo.png';
  import { ListSecrets, SetSecret, DeleteSecret, VaultExists, GetVaultDir } from '../wailsjs/go/main/App';

  let vaultReady = false;
  let secrets = [];
  let newKey = '';
  let newValue = '';
  let error = '';
  let revealed = {};
  let copied = {};
  let vaultDir = '';

  onMount(async () => {
    vaultDir = await GetVaultDir();
    vaultReady = await VaultExists();
    if (vaultReady) await refresh();
  });

  async function refresh() {
    try {
      secrets = await ListSecrets();
      error = '';
    } catch (e) {
      error = e.toString();
    }
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
      await refresh();
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
      await navigator.clipboard.writeText(text);
      copied[id] = true;
      copied = copied;
      setTimeout(() => {
        copied[id] = false;
        copied = copied;
      }, 1500);
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
      {#if secrets.length === 0}
        <p class="empty">No secrets stored yet.</p>
      {/if}
      {#each secrets as secret (secret.name)}
        <div class="secret-card">
          <div class="secret-header">
            <span class="secret-name">{secret.name}</span>
            <button class="delete" on:click={() => removeSecret(secret.name)}>Delete</button>
          </div>
          <div class="fields">
            {#each Object.entries(secret.data ?? {}) as [k, v] (k)}
              {@const rowId = secret.name + '/' + k}
              <div class="field-row">
                <span class="field-key">{k}</span>
                <input
                  type={revealed[rowId] ? 'text' : 'password'}
                  value={v}
                  readonly
                  class="field-value"
                />
                <button class="icon-btn" on:click={() => toggleReveal(rowId)} title={revealed[rowId] ? 'Hide' : 'Show'}>
                  {revealed[rowId] ? 'Hide' : 'Show'}
                </button>
                <button class="icon-btn" on:click={() => copyToClipboard(v, rowId)} title="Copy to clipboard">
                  {copied[rowId] ? 'Copied!' : 'Copy'}
                </button>
              </div>
            {/each}
          </div>
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

  .logo {
    width: 120px;
    margin-bottom: 1rem;
  }

  .init-screen h2 {
    color: #e85d04;
    margin-bottom: 0.5rem;
  }

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
    margin-bottom: 2rem;
    border-bottom: 1px solid #2a2a2a;
    padding-bottom: 1rem;
  }

  .header-logo {
    width: 36px;
    height: 36px;
    object-fit: contain;
  }

  header h1 {
    margin: 0;
    font-size: 1.4rem;
    color: #e85d04;
    letter-spacing: 0.02em;
  }

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

  .secrets-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    margin-bottom: 2rem;
  }

  .empty {
    color: #666;
    font-style: italic;
  }

  .secret-card {
    background: #1a1a1a;
    border: 1px solid #2a2a2a;
    border-radius: 8px;
    padding: 0.75rem 1rem;
  }

  .secret-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 0.5rem;
  }

  .secret-name {
    font-weight: 600;
    color: #e85d04;
    font-size: 1rem;
  }

  .fields {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .field-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .field-key {
    min-width: 80px;
    font-size: 0.8rem;
    color: #999;
    text-transform: uppercase;
    letter-spacing: 0.05em;
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

  button:hover {
    background: #ff6a1a;
  }

  button.delete {
    background: #7a1a1a;
  }

  button.delete:hover {
    background: #c0392b;
  }

  .icon-btn {
    background: #2a2a2a;
    min-width: 60px;
    padding: 0.3rem 0.6rem;
    font-size: 0.78rem;
  }

  .icon-btn:hover {
    background: #383838;
  }

  .vault-path {
    font-size: 0.72rem;
    color: #555;
    font-family: monospace;
    margin-bottom: 1rem;
    margin-top: -1.5rem;
  }

  .refresh-btn {
    margin-left: auto;
    background: #2a2a2a;
    color: #ccc;
    font-size: 1.2rem;
    padding: 0.2rem 0.6rem;
    border-radius: 4px;
  }

  .refresh-btn:hover {
    background: #3a3a3a;
    color: #fff;
  }

  .retry-btn {
    background: #c0392b;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .retry-btn:hover {
    background: #e74c3c;
  }

  .add-form {
    border-top: 1px solid #2a2a2a;
    padding-top: 1.5rem;
  }

  .add-form h3 {
    margin: 0 0 0.75rem;
    color: #ccc;
    font-size: 1rem;
    font-weight: 500;
  }

  .add-row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }

  .add-input {
    background: #1a1a1a;
    border: 1px solid #333;
    color: #f0f0f0;
    padding: 0.4rem 0.75rem;
    border-radius: 4px;
    font-size: 0.9rem;
    flex: 1;
  }

  .add-input:focus {
    outline: none;
    border-color: #e85d04;
  }
</style>
