package cmd

import (
	"strings"
	"testing"
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
	if err := setCmd.Args(setCmd, []string{"map", "MY_MAP", "k=v"}); err == nil {
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
	if err := setCmd.Args(setCmd, []string{"map", "MY_MAP", "host=localhost"}); err != nil {
		t.Errorf("expected no error for non-interactive map with 3 args, got: %v", err)
	}
}

// --- key=value parsing tests (RunE path) ---

func TestSetMap_ValidKVPairs(t *testing.T) {
	stub := newStub(nil)
	defer withStub(stub)()

	if err := executeSetViaRoot([]string{"map", "CFG", "host=localhost", "port=5432"}); err != nil {
		t.Fatalf("expected no error for valid k=v pairs, got: %v", err)
	}

	got, ok := stub.secrets["CFG"]
	if !ok {
		t.Fatal("expected secret CFG to be written")
	}
	if got.Data["host"] != "localhost" {
		t.Errorf("expected host=localhost, got %q", got.Data["host"])
	}
	if got.Data["port"] != "5432" {
		t.Errorf("expected port=5432, got %q", got.Data["port"])
	}
}

func TestSetMap_MissingEquals_Error(t *testing.T) {
	defer withStub(newStub(nil))()

	err := executeSetViaRoot([]string{"map", "CFG", "noequalssign"})
	if err == nil {
		t.Error("expected error for k=v pair missing '=', got nil")
	}
	if !strings.Contains(err.Error(), "missing '='") {
		t.Errorf("expected 'missing =' in error, got: %v", err)
	}
}

func TestSetEnvironment_WritesValueKey(t *testing.T) {
	stub := newStub(nil)
	defer withStub(stub)()

	if err := executeSetViaRoot([]string{"environment", "DB_URL", "postgres://localhost"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := stub.secrets["DB_URL"]
	if !ok {
		t.Fatal("expected secret DB_URL to be written")
	}
	if got.Data["value"] != "postgres://localhost" {
		t.Errorf("expected value=postgres://localhost, got %q", got.Data["value"])
	}
}
