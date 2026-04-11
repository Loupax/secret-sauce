package cmd

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/loupax/secret-sauce/internal/vault"
)

// captureGet runs the get subcommand via rootCmd and captures stdout.
func captureGet(args []string) (string, error) {
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	rootCmd.SetArgs(append([]string{"get"}, args...))
	err := rootCmd.Execute()

	w.Close()
	os.Stdout = origStdout

	out, _ := io.ReadAll(r)
	return string(out), err
}

func TestGetMap_NoKey_PrintsAllData(t *testing.T) {
	secrets := map[string]vault.SecretInfo{
		"CFG": {Data: map[string]string{"host": "localhost", "port": "5432"}},
	}
	defer withStub(newStub(secrets))()

	out, err := captureGet([]string{"CFG"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "host=localhost") {
		t.Errorf("expected host=localhost in output, got: %q", out)
	}
}

func TestGetMap_WithKey_PrintsRawValueNoNewline(t *testing.T) {
	secrets := map[string]vault.SecretInfo{
		"CFG": {Data: map[string]string{"host": "localhost", "port": "5432"}},
	}
	defer withStub(newStub(secrets))()

	out, err := captureGet([]string{"CFG", "host"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "localhost" {
		t.Errorf("expected %q, got %q", "localhost", out)
	}
}

func TestGetMap_MissingKey_Error(t *testing.T) {
	secrets := map[string]vault.SecretInfo{
		"CFG": {Data: map[string]string{"host": "localhost"}},
	}
	defer withStub(newStub(secrets))()

	_, err := captureGet([]string{"CFG", "missing"})
	if err == nil {
		t.Error("expected error for missing map key, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestGetEnvironment_NoKey_PrintsValue(t *testing.T) {
	secrets := map[string]vault.SecretInfo{
		"DB_URL": {Data: map[string]string{"value": "postgres://localhost"}},
	}
	defer withStub(newStub(secrets))()

	out, err := captureGet([]string{"DB_URL"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "postgres://localhost") {
		t.Errorf("expected value in output, got: %q", out)
	}
}

func TestGetFile_NoKey_PrintsValue(t *testing.T) {
	secrets := map[string]vault.SecretInfo{
		"MY_CERT": {Data: map[string]string{"value": "cert-contents"}},
	}
	defer withStub(newStub(secrets))()

	out, err := captureGet([]string{"MY_CERT"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "cert-contents") {
		t.Errorf("expected value in output, got: %q", out)
	}
}

func TestGetSecret_NotFound_Error(t *testing.T) {
	defer withStub(newStub(nil))()

	_, err := captureGet([]string{"NONEXISTENT"})
	if err == nil {
		t.Error("expected error for missing secret, got nil")
	}
}
