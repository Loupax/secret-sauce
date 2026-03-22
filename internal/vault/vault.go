package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"filippo.io/age"
)

var ErrKeyNotFound = errors.New("key not found")

func Exists(vaultDir string) bool {
	for _, name := range []string{"vault.age", "vault_recipients.txt"} {
		if _, err := os.Stat(filepath.Join(vaultDir, name)); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func Init(vaultDir string, identity *age.X25519Identity) error {
	if err := os.MkdirAll(vaultDir, 0700); err != nil {
		return fmt.Errorf("create vault dir: %w", err)
	}
	recipientsPath := filepath.Join(vaultDir, "vault_recipients.txt")
	if err := os.WriteFile(recipientsPath, []byte(identity.Recipient().String()+"\n"), 0600); err != nil {
		return fmt.Errorf("write recipients file: %w", err)
	}
	return Write(vaultDir, map[string]string{}, []age.Recipient{identity.Recipient()})
}

func Read(vaultDir string, identity age.Identity) (map[string]string, error) {
	vaultPath := filepath.Join(vaultDir, "vault.age")
	f, err := os.Open(vaultPath)
	if err != nil {
		return nil, fmt.Errorf("open vault: %w", err)
	}
	defer f.Close()

	r, err := age.Decrypt(f, identity)
	if err != nil {
		return nil, fmt.Errorf("create age reader: %w", err)
	}

	var m map[string]string
	if err := json.NewDecoder(r).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode vault: %w", err)
	}
	return m, nil
}

func Write(vaultDir string, secrets map[string]string, recipients []age.Recipient) error {
	tmp, err := os.CreateTemp(vaultDir, "vault-*.age.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	w, err := age.Encrypt(tmp, recipients...)
	if err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("create age writer: %w", err)
	}

	if err := json.NewEncoder(w).Encode(secrets); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("encode secrets: %w", err)
	}

	if err := w.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("close age writer: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("sync temp file: %w", err)
	}
	tmp.Close()

	dest := filepath.Join(vaultDir, "vault.age")
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return fmt.Errorf("rename vault file: %w", err)
	}
	return nil
}

