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
	if err := WriteSecret(vaultDir, "FOO", "bar", SecretTypeEnvironment, recipients, identity); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	got, err := ReadSecret(vaultDir, "FOO", identity)
	if err != nil {
		t.Fatalf("ReadSecret: %v", err)
	}
	if got.Value != "bar" {
		t.Errorf("ReadSecret: want %q, got %q", "bar", got.Value)
	}
	if got.Type != SecretTypeEnvironment {
		t.Errorf("ReadSecret: want type %q, got %q", SecretTypeEnvironment, got.Type)
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
	if err := WriteSecret(vaultDir, "FOO", "bar", SecretTypeEnvironment, recipients, identity); err != nil {
		t.Fatalf("WriteSecret (first): %v", err)
	}

	// Overwrite with new value
	if err := WriteSecret(vaultDir, "FOO", "updated", SecretTypeEnvironment, recipients, identity); err != nil {
		t.Fatalf("WriteSecret (overwrite): %v", err)
	}

	got, err := ReadSecret(vaultDir, "FOO", identity)
	if err != nil {
		t.Fatalf("ReadSecret after overwrite: %v", err)
	}
	if got.Value != "updated" {
		t.Errorf("ReadSecret after overwrite: want %q, got %q", "updated", got.Value)
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
	if err := WriteSecret(vaultDir, "TO_DELETE", "secret", SecretTypeEnvironment, recipients, identity); err != nil {
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
		if err := WriteSecret(vaultDir, k, v, SecretTypeEnvironment, recipients, identity); err != nil {
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
		if got[k].Value != want {
			t.Errorf("%s: want %q, got %q", k, want, got[k].Value)
		}
		if got[k].Type != SecretTypeEnvironment {
			t.Errorf("%s: want type %q, got %q", k, SecretTypeEnvironment, got[k].Type)
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

	if err := WriteSecret(vaultDir, "KEY", "value", SecretTypeEnvironment, recipients, identity1); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	got, err := ReadSecret(vaultDir, "KEY", identity2)
	if err != nil {
		t.Fatalf("ReadSecret with identity2: %v", err)
	}
	if got.Value != "value" {
		t.Errorf("KEY: want %q, got %q", "value", got.Value)
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

func TestValidSecretType(t *testing.T) {
	if !ValidSecretType(SecretTypeEnvironment) {
		t.Error("expected SecretTypeEnvironment to be valid")
	}
	if !ValidSecretType(SecretTypeFile) {
		t.Error("expected SecretTypeFile to be valid")
	}
	if ValidSecretType(SecretType("env_var")) {
		t.Error("expected 'env_var' to be invalid")
	}
	if ValidSecretType(SecretType("")) {
		t.Error("expected empty string to be invalid")
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
	if err := WriteSecret(vaultDir, "MY_CERT", "cert-contents", SecretTypeFile, recipients, identity); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	got, err := ReadSecret(vaultDir, "MY_CERT", identity)
	if err != nil {
		t.Fatalf("ReadSecret: %v", err)
	}
	if got.Value != "cert-contents" {
		t.Errorf("ReadSecret: want %q, got %q", "cert-contents", got.Value)
	}
	if got.Type != SecretTypeFile {
		t.Errorf("ReadSecret: want type %q, got %q", SecretTypeFile, got.Type)
	}
}
