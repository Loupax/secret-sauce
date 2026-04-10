package cmd

import (
	"strings"
	"testing"

	"github.com/loupax/secret-sauce/internal/vault"
)

// executeSetViaRoot runs the set subcommand through rootCmd.
func executeSetViaRoot(args []string) error {
	rootCmd.SetArgs(append([]string{"set"}, args...))
	return rootCmd.Execute()
}

// --- Args validator tests (test the Args func directly) ---

func TestSetArgsValidator_MapInteractive_TwoArgs_Valid(t *testing.T) {
	interactive = true
	defer func() { interactive = false }()
	if err := setCmd.Args(setCmd, []string{"map", "MY_MAP"}); err != nil {
		t.Errorf("expected no error for interactive map with 2 args, got: %v", err)
	}
}

func TestSetArgsValidator_MapInteractive_ThreeArgs_Error(t *testing.T) {
	interactive = true
	defer func() { interactive = false }()
	if err := setCmd.Args(setCmd, []string{"map", "MY_MAP", `{"a":"b"}`}); err == nil {
		t.Error("expected error for interactive map with 3 args, got nil")
	}
}

func TestSetArgsValidator_MapNonInteractive_TwoArgs_Error(t *testing.T) {
	interactive = false
	if err := setCmd.Args(setCmd, []string{"map", "MY_MAP"}); err == nil {
		t.Error("expected error for non-interactive map with 2 args, got nil")
	}
}

func TestSetArgsValidator_MapNonInteractive_ThreeArgs_Valid(t *testing.T) {
	interactive = false
	if err := setCmd.Args(setCmd, []string{"map", "MY_MAP", `{"a":"b"}`}); err != nil {
		t.Errorf("expected no error for non-interactive map with 3 args, got: %v", err)
	}
}

// --- JSON validation tests (RunE path) ---

func TestSetMap_ValidFlatJSON(t *testing.T) {
	stub := newStub(nil)
	defer withStub(stub)()

	if err := executeSetViaRoot([]string{"map", "CFG", `{"host":"localhost","port":"5432"}`}); err != nil {
		t.Fatalf("expected no error for valid flat JSON, got: %v", err)
	}

	got, ok := stub.secrets["CFG"]
	if !ok {
		t.Fatal("expected secret CFG to be written")
	}
	if got.Type != vault.SecretTypeMap {
		t.Errorf("expected type map, got %q", got.Type)
	}
}

func TestSetMap_NestedJSON_Error(t *testing.T) {
	defer withStub(newStub(nil))()

	err := executeSetViaRoot([]string{"map", "CFG", `{"a":{"b":"c"}}`})
	if err == nil {
		t.Error("expected error for nested JSON, got nil")
	}
	if !strings.Contains(err.Error(), "non-string value") {
		t.Errorf("expected 'non-string value' in error, got: %v", err)
	}
}

func TestSetMap_InvalidJSON_Error(t *testing.T) {
	defer withStub(newStub(nil))()

	err := executeSetViaRoot([]string{"map", "CFG", "not-json"})
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("expected 'invalid JSON' in error, got: %v", err)
	}
}

func TestSetMap_ArrayValue_Error(t *testing.T) {
	defer withStub(newStub(nil))()

	err := executeSetViaRoot([]string{"map", "CFG", `{"items":["a","b"]}`})
	if err == nil {
		t.Error("expected error for JSON with array value, got nil")
	}
	if !strings.Contains(err.Error(), "non-string value") {
		t.Errorf("expected 'non-string value' in error, got: %v", err)
	}
}
