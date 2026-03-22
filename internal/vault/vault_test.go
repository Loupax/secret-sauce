package vault

import (
	"os"
	"testing"

	"filippo.io/age"
)

func TestInitAndRead(t *testing.T) {
	vaultDir := t.TempDir()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	if err := Init(vaultDir, identity); err != nil {
		t.Fatalf("Init: %v", err)
	}

	m, err := Read(vaultDir, identity)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil map")
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(m))
	}
}

func TestWriteAndRead(t *testing.T) {
	vaultDir := t.TempDir()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	if err := Init(vaultDir, identity); err != nil {
		t.Fatalf("Init: %v", err)
	}

	secrets := map[string]string{"FOO": "bar", "BAZ": "qux"}
	if err := Write(vaultDir, secrets, []age.Recipient{identity.Recipient()}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	m, err := Read(vaultDir, identity)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if m["FOO"] != "bar" {
		t.Errorf("FOO: want %q, got %q", "bar", m["FOO"])
	}
	if m["BAZ"] != "qux" {
		t.Errorf("BAZ: want %q, got %q", "qux", m["BAZ"])
	}
}

func TestMultiRecipient(t *testing.T) {
	vaultDir := t.TempDir()

	identity1, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity1: %v", err)
	}
	identity2, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity2: %v", err)
	}

	if err := Init(vaultDir, identity1); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := AppendRecipient(vaultDir, identity2.Recipient().String()); err != nil {
		t.Fatalf("AppendRecipient: %v", err)
	}

	recipients, err := ReadRecipients(vaultDir)
	if err != nil {
		t.Fatalf("ReadRecipients: %v", err)
	}

	secrets := map[string]string{"KEY": "value"}
	if err := Write(vaultDir, secrets, recipients); err != nil {
		t.Fatalf("Write: %v", err)
	}

	m, err := Read(vaultDir, identity2)
	if err != nil {
		t.Fatalf("Read with identity2: %v", err)
	}
	if m["KEY"] != "value" {
		t.Errorf("KEY: want %q, got %q", "value", m["KEY"])
	}
}

func TestReadRecipients(t *testing.T) {
	vaultDir := t.TempDir()

	identity1, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity1: %v", err)
	}
	identity2, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity2: %v", err)
	}

	content := identity1.Recipient().String() + "\n" + identity2.Recipient().String() + "\n"
	recipientsPath := vaultDir + "/vault_recipients.txt"
	if err := os.WriteFile(recipientsPath, []byte(content), 0600); err != nil {
		t.Fatalf("write recipients file: %v", err)
	}

	recipients, err := ReadRecipients(vaultDir)
	if err != nil {
		t.Fatalf("ReadRecipients: %v", err)
	}
	if len(recipients) != 2 {
		t.Fatalf("expected 2 recipients, got %d", len(recipients))
	}
}

func TestExists(t *testing.T) {
	vaultDir := t.TempDir()

	if Exists(vaultDir) {
		t.Fatal("expected Exists to return false on empty dir")
	}

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	if err := Init(vaultDir, identity); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !Exists(vaultDir) {
		t.Fatal("expected Exists to return true after Init")
	}
}
