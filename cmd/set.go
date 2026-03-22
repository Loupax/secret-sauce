package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set KEY VALUE",
	Short: "Set a secret",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := resolveService()
		if err != nil {
			return fmt.Errorf("resolve service: %w", err)
		}

		if err := svc.WriteSecret(vaultDir, args[0], args[1]); err != nil {
			return fmt.Errorf("failed to write secret: %w", err)
		}

		return nil
	},
}
