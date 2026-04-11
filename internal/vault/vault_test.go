package vault

import (
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestInitAndExists(t *testing.T) {
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

	// Verify .vault_recipients was created
	if _, err := os.Stat(vaultDir + "/.vault_recipients"); err != nil {
		t.Fatalf("expected .vault_recipients to exist: %v", err)
	}
}

func TestWriteAndReadSecret(t *testing.T) {
	vaultDir := t.TempDir()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	if err := Init(vaultDir, identity); err != nil {
		t.Fatalf("Init: %v", err)
	}

	recipients := []age.Recipient{identity.Recipient()}
	data := map[string]string{"value": "bar"}
	if err := WriteSecret(vaultDir, "FOO", data, recipients, identity); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	got, err := ReadSecret(vaultDir, "FOO", identity)
	if err != nil {
		t.Fatalf("ReadSecret: %v", err)
	}
	if got.Data["value"] != "bar" {
		t.Errorf("ReadSecret: want %q, got %q", "bar", got.Data["value"])
	}
}

func TestWriteSecretOverwrite(t *testing.T) {
	vaultDir := t.TempDir()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	if err := Init(vaultDir, identity); err != nil {
		t.Fatalf("Init: %v", err)
	}

	recipients := []age.Recipient{identity.Recipient()}

	// Write initial value
	if err := WriteSecret(vaultDir, "FOO", map[string]string{"value": "bar"}, recipients, identity); err != nil {
		t.Fatalf("WriteSecret (first): %v", err)
	}

	// Overwrite with new value
	if err := WriteSecret(vaultDir, "FOO", map[string]string{"value": "updated"}, recipients, identity); err != nil {
		t.Fatalf("WriteSecret (overwrite): %v", err)
	}

	got, err := ReadSecret(vaultDir, "FOO", identity)
	if err != nil {
		t.Fatalf("ReadSecret after overwrite: %v", err)
	}
	if got.Data["value"] != "updated" {
		t.Errorf("ReadSecret after overwrite: want %q, got %q", "updated", got.Data["value"])
	}

	// Verify only one .age file exists (overwrite, not duplicate)
	files, err := filepath.Glob(filepath.Join(vaultDir, "*.age"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 .age file after overwrite, got %d", len(files))
	}
}

func TestWriteSecretStrictReplace(t *testing.T) {
	vaultDir := t.TempDir()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	if err := Init(vaultDir, identity); err != nil {
		t.Fatalf("Init: %v", err)
	}

	recipients := []age.Recipient{identity.Recipient()}

	// Write initial data with two keys
	if err := WriteSecret(vaultDir, "FOO", map[string]string{"a": "1", "b": "2"}, recipients, identity); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	// Overwrite with only one key — the other must be dropped
	if err := WriteSecret(vaultDir, "FOO", map[string]string{"a": "new"}, recipients, identity); err != nil {
		t.Fatalf("WriteSecret (overwrite): %v", err)
	}

	got, err := ReadSecret(vaultDir, "FOO", identity)
	if err != nil {
		t.Fatalf("ReadSecret: %v", err)
	}
	if got.Data["a"] != "new" {
		t.Errorf("expected a=new, got %q", got.Data["a"])
	}
	if _, exists := got.Data["b"]; exists {
		t.Error("expected key 'b' to be dropped after strict replace, but it still exists")
	}
}

func TestReadSecretNotFound(t *testing.T) {
	vaultDir := t.TempDir()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	_, err = ReadSecret(vaultDir, "NONEXISTENT", identity)
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got: %v", err)
	}
}

func TestDeleteSecret(t *testing.T) {
	vaultDir := t.TempDir()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	if err := Init(vaultDir, identity); err != nil {
		t.Fatalf("Init: %v", err)
	}

	recipients := []age.Recipient{identity.Recipient()}
	if err := WriteSecret(vaultDir, "TO_DELETE", map[string]string{"value": "secret"}, recipients, identity); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	if err := DeleteSecret(vaultDir, "TO_DELETE", identity); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}

	_, err = ReadSecret(vaultDir, "TO_DELETE", identity)
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound after delete, got: %v", err)
	}
}

func TestDeleteSecretNotFound(t *testing.T) {
	vaultDir := t.TempDir()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	err = DeleteSecret(vaultDir, "NONEXISTENT", identity)
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got: %v", err)
	}
}

func TestReadAllSecrets(t *testing.T) {
	vaultDir := t.TempDir()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	if err := Init(vaultDir, identity); err != nil {
		t.Fatalf("Init: %v", err)
	}

	recipients := []age.Recipient{identity.Recipient()}

	secrets := map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
		"DB":  "postgres://localhost",
	}
	for k, v := range secrets {
		if err := WriteSecret(vaultDir, k, map[string]string{"value": v}, recipients, identity); err != nil {
			t.Fatalf("WriteSecret(%s): %v", k, err)
		}
	}

	got, err := ReadAllSecrets(vaultDir, identity)
	if err != nil {
		t.Fatalf("ReadAllSecrets: %v", err)
	}

	if len(got) != len(secrets) {
		t.Fatalf("expected %d secrets, got %d", len(secrets), len(got))
	}

	for k, want := range secrets {
		if got[k].Data["value"] != want {
			t.Errorf("%s: want %q, got %q", k, want, got[k].Data["value"])
		}
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

	if err := WriteSecret(vaultDir, "KEY", map[string]string{"value": "value"}, recipients, identity1); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	got, err := ReadSecret(vaultDir, "KEY", identity2)
	if err != nil {
		t.Fatalf("ReadSecret with identity2: %v", err)
	}
	if got.Data["value"] != "value" {
		t.Errorf("KEY: want %q, got %q", "value", got.Data["value"])
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
	recipientsPath := vaultDir + "/.vault_recipients"
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

func TestFileTypeSecret(t *testing.T) {
	vaultDir := t.TempDir()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	if err := Init(vaultDir, identity); err != nil {
		t.Fatalf("Init: %v", err)
	}

	recipients := []age.Recipient{identity.Recipient()}
	if err := WriteSecret(vaultDir, "MY_CERT", map[string]string{"value": "cert-contents"}, recipients, identity); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	got, err := ReadSecret(vaultDir, "MY_CERT", identity)
	if err != nil {
		t.Fatalf("ReadSecret: %v", err)
	}
	if got.Data["value"] != "cert-contents" {
		t.Errorf("ReadSecret: want %q, got %q", "cert-contents", got.Data["value"])
	}
}
