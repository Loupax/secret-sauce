package service

// VaultService is the Strategy interface. Both the IPC client and local
// execution path implement this. Commands (run, set, rm) accept a VaultService.
type VaultService interface {
	ReadAllSecrets(vaultDir string) (map[string]string, error)
	WriteSecret(vaultDir, key, value string) error
	DeleteSecret(vaultDir, key string) error
}
