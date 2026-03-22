package keyring

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const serviceName = "secret-sauce"

var ErrNoSecretService = errors.New(`no Secret Service provider found on D-Bus.
Start one before using this tool:
  KeePassXC:  keepassxc &  (enable Secret Service integration in settings)
  GNOME:      /usr/lib/gnome-keyring-daemon --start`)

func serviceUser(vaultDir string) (service, user string) {
	service = serviceName
	user = fmt.Sprintf("%x", sha256.Sum256([]byte(vaultDir)))
	return
}

func isDBusError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "secret service") ||
		strings.Contains(msg, "dbus") ||
		strings.Contains(msg, "org.freedesktop") ||
		strings.Contains(msg, "no such interface") ||
		strings.Contains(msg, "the name org.freedesktop") ||
		strings.Contains(msg, "not activatable") ||
		strings.Contains(msg, "not available")
}

func Save(vaultDir, privateKey string) error {
	service, user := serviceUser(vaultDir)
	err := keyring.Set(service, user, privateKey)
	if err == nil {
		return nil
	}
	if isDBusError(err) {
		return ErrNoSecretService
	}
	return fmt.Errorf("keyring save: %w", err)
}

func Load(vaultDir string) (string, error) {
	service, user := serviceUser(vaultDir)
	val, err := keyring.Get(service, user)
	if err == nil {
		return val, nil
	}
	if isDBusError(err) {
		return "", ErrNoSecretService
	}
	return "", fmt.Errorf("keyring load: %w", err)
}

func Delete(vaultDir string) error {
	service, user := serviceUser(vaultDir)
	err := keyring.Delete(service, user)
	if err == nil {
		return nil
	}
	if isDBusError(err) {
		return ErrNoSecretService
	}
	return fmt.Errorf("keyring delete: %w", err)
}
