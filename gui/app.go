package main

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/loupax/secret-sauce/pkg/guisvc"
)

// SecretEntry is the JSON-serializable view sent to the frontend.
type SecretEntry struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

// App is the Wails application struct. All exported methods on App are callable from JS.
type App struct {
	ctx      context.Context
	mu       sync.Mutex
	svc      guisvc.VaultService
	vaultDir string
}

func NewApp() *App {
	return &App{}
}

// startup is called by the Wails runtime when the application starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	vaultDir, err := guisvc.ResolveVaultDir()
	if err != nil {
		runtime.LogErrorf(ctx, "vault dir resolution failed: %v", err)
		return
	}
	a.vaultDir = vaultDir

	svc, err := guisvc.ResolveService()
	if err != nil {
		runtime.LogErrorf(ctx, "service resolution failed: %v", err)
		return
	}
	a.svc = svc
}

func (a *App) ready() error {
	if a.svc == nil || a.vaultDir == "" {
		return fmt.Errorf("vault service not ready — keychain may be locked or vault not initialized")
	}
	return nil
}

func (a *App) quit() {
	runtime.Quit(a.ctx)
}

// VaultExists reports whether the vault directory has been initialized.
func (a *App) VaultExists() bool {
	return guisvc.Exists(a.vaultDir)
}

// GetVaultDir returns the resolved vault directory path.
func (a *App) GetVaultDir() string {
	return a.vaultDir
}

// ListSecretNames returns all secret names sorted alphabetically.
// Values are not decrypted — call GetSecret for a specific secret.
func (a *App) ListSecretNames() ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ready(); err != nil {
		return nil, err
	}

	return a.svc.ListSecretNames(a.vaultDir)
}

// ListSecrets returns all secrets sorted by name.
func (a *App) ListSecrets() ([]SecretEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ready(); err != nil {
		return nil, err
	}

	all, err := a.svc.ReadAllSecrets(a.vaultDir)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	entries := make([]SecretEntry, 0, len(all))
	for name, info := range all {
		if info.Data == nil {
			info.Data = map[string]string{}
		}
		entries = append(entries, SecretEntry{Name: name, Data: info.Data})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// GetSecret returns a single secret by name.
func (a *App) GetSecret(key string) (SecretEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ready(); err != nil {
		return SecretEntry{}, err
	}

	info, err := a.svc.ReadSecret(a.vaultDir, key)
	if err != nil {
		return SecretEntry{}, fmt.Errorf("get secret %q: %w", key, err)
	}
	return SecretEntry{Name: key, Data: info.Data}, nil
}

// SetSecret writes a secret. data is a field-name → value map.
func (a *App) SetSecret(key string, data map[string]string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ready(); err != nil {
		return err
	}

	if err := a.svc.WriteSecret(a.vaultDir, key, data); err != nil {
		return fmt.Errorf("set secret %q: %w", key, err)
	}
	return nil
}

// DeleteSecret removes a secret by name.
func (a *App) DeleteSecret(key string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ready(); err != nil {
		return err
	}

	if err := a.svc.DeleteSecret(a.vaultDir, key); err != nil {
		return fmt.Errorf("delete secret %q: %w", key, err)
	}
	return nil
}
