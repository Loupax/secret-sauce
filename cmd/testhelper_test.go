package cmd

import (
	"github.com/loupax/secret-sauce/internal/service"
	"github.com/loupax/secret-sauce/internal/vault"
)

// stubVaultService is a minimal in-memory VaultService for tests.
type stubVaultService struct {
	secrets map[string]vault.SecretInfo
}

func newStub(secrets map[string]vault.SecretInfo) *stubVaultService {
	return &stubVaultService{secrets: secrets}
}

func (s *stubVaultService) ListSecretNames(_ string) ([]string, error) {
	names := make([]string, 0, len(s.secrets))
	for name := range s.secrets {
		names = append(names, name)
	}
	return names, nil
}

func (s *stubVaultService) ReadAllSecrets(_ string) (map[string]vault.SecretInfo, error) {
	return s.secrets, nil
}

func (s *stubVaultService) ReadSecret(_ string, key string) (vault.SecretInfo, error) {
	info, ok := s.secrets[key]
	if !ok {
		return vault.SecretInfo{}, vault.ErrKeyNotFound
	}
	return info, nil
}

func (s *stubVaultService) WriteSecret(_ string, key string, data map[string]string) error {
	if s.secrets == nil {
		s.secrets = make(map[string]vault.SecretInfo)
	}
	s.secrets[key] = vault.SecretInfo{Data: data}
	return nil
}

func (s *stubVaultService) DeleteSecret(_ string, key string) error {
	delete(s.secrets, key)
	return nil
}

func (s *stubVaultService) GetPublicKey(_ string) (string, error) {
	return "age1testpubkey", nil
}

// withStub replaces resolveService for the duration of a test and returns a
// restore function that must be deferred.
func withStub(svc service.VaultService) func() {
	orig := resolveService
	resolveService = func() (service.VaultService, error) { return svc, nil }
	return func() { resolveService = orig }
}
