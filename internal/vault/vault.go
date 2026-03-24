package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"filippo.io/age"
	"golang.org/x/sync/errgroup"
)

var ErrKeyNotFound = errors.New("key not found")

// SecretInfo is the decrypted representation returned to callers of ReadSecret / ReadAllSecrets.
type SecretInfo struct {
	Type  SecretType
	Value string
}

func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if key == "." || key == ".." {
		return fmt.Errorf("invalid key %q", key)
	}
	for _, c := range key {
		if c == '/' || c == '\\' {
			return fmt.Errorf("key %q cannot contain path separators", key)
		}
	}
	return nil
}

func Exists(vaultDir string) bool {
	if _, err := os.Stat(filepath.Join(vaultDir, ".vault_recipients")); os.IsNotExist(err) {
		return false
	}
	return true
}

func Init(vaultDir string, identity *age.X25519Identity) error {
	if err := os.MkdirAll(vaultDir, 0700); err != nil {
		return fmt.Errorf("create vault dir: %w", err)
	}
	recipientsPath := filepath.Join(vaultDir, ".vault_recipients")
	if err := os.WriteFile(recipientsPath, []byte(identity.Recipient().String()+"\n"), 0600); err != nil {
		return fmt.Errorf("write recipients file: %w", err)
	}
	return nil
}

// encryptToFile writes a JSON-marshaled envelope to destPath using an atomic
// temp→rename pattern.
func encryptToFile(destPath string, envelope SecretEnvelope, recipients []age.Recipient) error {
	uuidBase := filepath.Base(destPath)
	uuidBase = uuidBase[:len(uuidBase)-len(".age")] // strip .age suffix

	tmp, err := os.CreateTemp(filepath.Dir(destPath), uuidBase+"-*.age.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	w, err := age.Encrypt(tmp, recipients...)
	if err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("create age writer: %w", err)
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("marshal envelope: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("write secret: %w", err)
	}

	if err := w.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("close age writer: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmp.Name(), destPath); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("rename secret file: %w", err)
	}
	return nil
}

// WriteSecret writes key=value as a JSON envelope into a UUID-named .age file.
// If a file with the same envelope.Name already exists (found by decrypting
// with identity), it is overwritten in-place (same UUID filename). Otherwise a
// new UUID is generated.
func WriteSecret(vaultDir, key, value string, secretType SecretType, recipients []age.Recipient, identity age.Identity) error {
	if err := validateKey(key); err != nil {
		return err
	}

	// Search for an existing .age file whose envelope.Name matches key.
	existingPath := ""
	if identity != nil {
		pattern := filepath.Join(vaultDir, "*.age")
		files, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("glob secrets: %w", err)
		}
		for _, f := range files {
			env, err := DecryptEnvelope(f, identity)
			if err != nil {
				continue // skip corrupt/unreadable files
			}
			if env.Name == key {
				existingPath = f
				break
			}
		}
	}

	var destPath string
	if existingPath != "" {
		destPath = existingPath
	} else {
		uuidStr := newUUID()
		destPath = filepath.Join(vaultDir, uuidStr+".age")
	}

	envelope := SecretEnvelope{
		Type:  secretType,
		Name:  key,
		Value: value,
		Tags:  []string{},
	}

	return encryptToFile(destPath, envelope, recipients)
}

func ReadSecret(vaultDir, key string, identity age.Identity) (SecretInfo, error) {
	if err := validateKey(key); err != nil {
		return SecretInfo{}, err
	}

	files, err := filepath.Glob(filepath.Join(vaultDir, "*.age"))
	if err != nil {
		return SecretInfo{}, fmt.Errorf("glob secrets: %w", err)
	}

	for _, f := range files {
		env, err := DecryptEnvelope(f, identity)
		if err != nil {
			continue // skip corrupt/unreadable files
		}
		if env.Name == key {
			return SecretInfo{Type: env.Type, Value: env.Value}, nil
		}
	}
	return SecretInfo{}, ErrKeyNotFound
}

func ReadAllSecrets(vaultDir string, identity age.Identity) (map[string]SecretInfo, error) {
	matches, err := filepath.Glob(filepath.Join(vaultDir, "*.age"))
	if err != nil {
		return nil, fmt.Errorf("glob secrets: %w", err)
	}

	result := make(map[string]SecretInfo, len(matches))
	var mu sync.Mutex
	var g errgroup.Group

	for _, match := range matches {
		g.Go(func() error {
			env, err := DecryptEnvelope(match, identity)
			if err != nil {
				return fmt.Errorf("decrypt %s: %w", filepath.Base(match), err)
			}
			mu.Lock()
			result[env.Name] = SecretInfo{Type: env.Type, Value: env.Value}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteSecret removes the .age file whose envelope.Name matches key.
func DeleteSecret(vaultDir, key string, identity age.Identity) error {
	if err := validateKey(key); err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(vaultDir, "*.age"))
	if err != nil {
		return fmt.Errorf("glob secrets: %w", err)
	}

	for _, f := range files {
		env, err := DecryptEnvelope(f, identity)
		if err != nil {
			continue // skip corrupt/unreadable files
		}
		if env.Name == key {
			if err := os.Remove(f); err != nil {
				return fmt.Errorf("remove secret file: %w", err)
			}
			return nil
		}
	}
	return ErrKeyNotFound
}
