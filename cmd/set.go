package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	vlt "github.com/loupax/secret-sauce/internal/vault"
)

var setCmd = &cobra.Command{
	Use:   "set KEY VALUE",
	Short: "Set a secret",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		unlock, err := vlt.AcquireExclusive(vaultDir)
		if err != nil {
			return fmt.Errorf("failed to acquire vault lock: %w", err)
		}
		defer unlock()

		recipients, err := vlt.ReadRecipients(vaultDir)
		if err != nil {
			return fmt.Errorf("failed to read recipients: %w", err)
		}

		if err := vlt.WriteSecret(vaultDir, args[0], args[1], recipients); err != nil {
			return fmt.Errorf("failed to write secret: %w", err)
		}

		return nil
	},
}
