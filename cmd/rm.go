package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	vlt "github.com/loupax/secret-sauce/internal/vault"
)

var rmCmd = &cobra.Command{
	Use:   "rm KEY",
	Short: "Remove a secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		unlock, err := vlt.AcquireExclusive(vaultDir)
		if err != nil {
			return fmt.Errorf("failed to acquire vault lock: %w", err)
		}
		defer unlock()

		if err := vlt.DeleteSecret(vaultDir, args[0]); err != nil {
			return err
		}

		return nil
	},
}
