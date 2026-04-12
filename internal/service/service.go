package service

import "github.com/loupax/secret-sauce/internal/vault"

// VaultService is the Strategy interface. Both the IPC client and local
// execution path implement this. Commands (run, set, rm) accept a VaultService.
type VaultService interface {
	ListSecretNames(vaultDir string) ([]string, error)
	ReadAllSecrets(vaultDir string) (map[string]vault.SecretInfo, error)
	ReadSecret(vaultDir, key string) (vault.SecretInfo, error)
	WriteSecret(vaultDir, key string, data map[string]string) error
	DeleteSecret(vaultDir, key string) error
	GetPublicKey(vaultDir string) (string, error)
}
