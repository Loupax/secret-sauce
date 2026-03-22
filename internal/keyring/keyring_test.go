package keyring

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	m.Run()
}

func TestSaveAndLoad(t *testing.T) {
	vaultDir := "/tmp/test-vault-saveload"
	privateKey := "age-secret-key-1abc123"

	if err := Save(vaultDir, privateKey); err != nil {
		t.Fatalf("Save returned unexpected error: %v", err)
	}

	got, err := Load(vaultDir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if got != privateKey {
		t.Errorf("Load returned %q, want %q", got, privateKey)
	}
}

func TestLoadNotFound(t *testing.T) {
	vaultDir := "/tmp/test-vault-notfound-unique-xyz"

	_, err := Load(vaultDir)
	if err == nil {
		t.Fatal("Load expected an error for a key that was never saved, got nil")
	}
	if errors.Is(err, ErrNoSecretService) {
		t.Errorf("Load returned ErrNoSecretService, want a different error (e.g. not found)")
	}
}

func TestDBusErrorSentinel(t *testing.T) {
	dbusErr := errors.New("org.freedesktop.DBus.Error.ServiceUnknown")
	if !isDBusError(dbusErr) {
		t.Errorf("isDBusError(%q) = false, want true", dbusErr.Error())
	}

	plainErr := errors.New("some other error")
	if isDBusError(plainErr) {
		t.Errorf("isDBusError(%q) = true, want false", plainErr.Error())
	}

	if isDBusError(nil) {
		t.Error("isDBusError(nil) = true, want false")
	}
}
