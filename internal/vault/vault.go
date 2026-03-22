package vault

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"filippo.io/age"
	"golang.org/x/sync/errgroup"
)

var ErrKeyNotFound = errors.New("key not found")

func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if key == "." || key == ".." {
		return fmt.Errorf("invalid key %q", key)
	}
	if strings.ContainsAny(key, "/\\") {
		return fmt.Errorf("key %q cannot contain path separators", key)
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

func WriteSecret(vaultDir, key, value string, recipients []age.Recipient) error {
	if err := validateKey(key); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(vaultDir, key+"-*.age.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	w, err := age.Encrypt(tmp, recipients...)
	if err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("create age writer: %w", err)
	}

	if _, err := io.WriteString(w, value); err != nil {
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
	tmp.Close()

	dest := filepath.Join(vaultDir, key+".age")
	if err := os.Rename(tmp.Name(), dest); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("rename secret file: %w", err)
	}
	return nil
}

func ReadSecret(vaultDir, key string, identity age.Identity) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	path := filepath.Join(vaultDir, key+".age")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrKeyNotFound
		}
		return "", fmt.Errorf("open secret file: %w", err)
	}
	defer f.Close()

	r, err := age.Decrypt(f, identity)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read secret: %w", err)
	}
	return string(data), nil
}

func ReadAllSecrets(vaultDir string, identity age.Identity) (map[string]string, error) {
	matches, err := filepath.Glob(filepath.Join(vaultDir, "*.age"))
	if err != nil {
		return nil, fmt.Errorf("glob secrets: %w", err)
	}

	result := make(map[string]string, len(matches))
	var mu sync.Mutex
	var g errgroup.Group

	for _, match := range matches {
		match := match
		g.Go(func() error {
			base := filepath.Base(match)
			key := strings.TrimSuffix(base, ".age")

			f, err := os.Open(match)
			if err != nil {
				return fmt.Errorf("open %s: %w", base, err)
			}
			defer f.Close()

			r, err := age.Decrypt(f, identity)
			if err != nil {
				return fmt.Errorf("decrypt %s: %w", base, err)
			}

			data, err := io.ReadAll(r)
			if err != nil {
				return fmt.Errorf("read %s: %w", base, err)
			}

			mu.Lock()
			result[key] = string(data)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return result, nil
}

func DeleteSecret(vaultDir, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	path := filepath.Join(vaultDir, key+".age")
	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrKeyNotFound
		}
		return fmt.Errorf("remove secret file: %w", err)
	}
	return nil
}
