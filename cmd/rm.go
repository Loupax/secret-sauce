package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm KEY",
	Short: "Remove a secret",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		svc, err := resolveService()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		secrets, err := svc.ReadAllSecrets(vaultDir)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		keys := make([]string, 0, len(secrets))
		for k := range secrets {
			keys = append(keys, k)
		}
		return keys, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := resolveService()
		if err != nil {
			return fmt.Errorf("resolve service: %w", err)
		}

		if err := svc.DeleteSecret(vaultDir, args[0]); err != nil {
			return fmt.Errorf("failed to delete secret: %w", err)
		}

		return nil
	},
}
