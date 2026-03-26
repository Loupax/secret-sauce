package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/loupax/sauce/internal/vault"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit <type> <key>",
	Short: "Edit a secret in $EDITOR",
	Args:  cobra.ExactArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return []string{"environment", "file"}, cobra.ShellCompDirectiveNoFileComp
		case 1:
			return completeSecretKeys(cmd, args, toComplete)
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		secretType := vault.SecretType(args[0])
		if !vault.ValidSecretType(secretType) {
			return fmt.Errorf("type must be 'environment' or 'file'; got %q", args[0])
		}
		key := args[1]

		svc, err := resolveService()
		if err != nil {
			return fmt.Errorf("resolve service: %w", err)
		}

		// Read current value; if not found, start with empty string.
		var currentValue string
		info, err := svc.ReadSecret(vaultDir, key)
		if err != nil && !errors.Is(err, vault.ErrKeyNotFound) {
			return fmt.Errorf("read secret: %w", err)
		}
		if err == nil {
			currentValue = info.Value
		}

		// Create a temp file for the editor.
		tmp, err := os.CreateTemp("", "secret-sauce-edit-*")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
		defer os.Remove(tmp.Name())

		if err := tmp.Chmod(0600); err != nil {
			return fmt.Errorf("chmod temp file: %w", err)
		}

		if _, err := tmp.WriteString(currentValue); err != nil {
			return fmt.Errorf("write temp file: %w", err)
		}
		if err := tmp.Close(); err != nil {
			return fmt.Errorf("close temp file: %w", err)
		}

		// Determine editor binary.
		editor := os.Getenv("EDITOR")
		if editor == "" {
			for _, candidate := range []string{"vi", "nano"} {
				if path, err := exec.LookPath(candidate); err == nil {
					editor = path
					break
				}
			}
		}
		if editor == "" {
			return fmt.Errorf("no editor found: set $EDITOR or install vi/nano")
		}

		// Launch editor.
		c := exec.Command(editor, tmp.Name())
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("editor exited with error: %w", err)
		}

		// Re-read the temp file.
		contents, err := os.ReadFile(tmp.Name())
		if err != nil {
			return fmt.Errorf("read temp file after edit: %w", err)
		}

		// Persist the updated value.
		if err := svc.WriteSecret(vaultDir, key, string(contents), secretType); err != nil {
			return fmt.Errorf("write secret: %w", err)
		}

		return nil
	},
}
