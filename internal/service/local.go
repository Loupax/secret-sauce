package service

import (
	"fmt"

	"filippo.io/age"
	"github.com/loupax/secret-sauce/internal/keyring"
	"github.com/loupax/secret-sauce/internal/vault"
)

type LocalVaultService struct{}

func NewLocalVaultService() *LocalVaultService { return &LocalVaultService{} }

func (s *LocalVaultService) loadIdentity(vaultDir string) (age.Identity, error) {
	keyStr, err := keyring.Load(vaultDir)
	if err != nil {
		return nil, fmt.Errorf("load private key: %w", err)
	}
	identity, err := age.ParseX25519Identity(keyStr)
	if err != nil {
		return nil, fmt.Errorf("parse identity: %w", err)
	}
	return identity, nil
}

func (s *LocalVaultService) ReadAllSecrets(vaultDir string) (map[string]string, error) {
	identity, err := s.loadIdentity(vaultDir)
	if err != nil {
		return nil, err
	}

	unlock, err := vault.AcquireShared(vaultDir)
	if err != nil {
		return nil, fmt.Errorf("acquire shared lock: %w", err)
	}
	defer unlock()

	return vault.ReadAllSecrets(vaultDir, identity)
}

func (s *LocalVaultService) WriteSecret(vaultDir, key, value string) error {
	identity, err := s.loadIdentity(vaultDir)
	if err != nil {
		return err
	}

	unlock, err := vault.AcquireExclusive(vaultDir)
	if err != nil {
		return fmt.Errorf("acquire exclusive lock: %w", err)
	}
	defer unlock()

	recipients, err := vault.ReadRecipients(vaultDir)
	if err != nil {
		return fmt.Errorf("read recipients: %w", err)
	}

	return vault.WriteSecret(vaultDir, key, value, recipients, identity)
}

func (s *LocalVaultService) DeleteSecret(vaultDir, key string) error {
	identity, err := s.loadIdentity(vaultDir)
	if err != nil {
		return err
	}

	unlock, err := vault.AcquireExclusive(vaultDir)
	if err != nil {
		return fmt.Errorf("acquire exclusive lock: %w", err)
	}
	defer unlock()

	return vault.DeleteSecret(vaultDir, key, identity)
}
