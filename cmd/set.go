package cmd

import (
	"fmt"

	"github.com/loupax/secret-sauce/internal/vault"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <type> <key> <value>",
	Short: "Set a secret",
	Args:  cobra.ExactArgs(3),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return []string{"environment", "file"}, cobra.ShellCompDirectiveNoFileComp
		case 1:
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
		value := args[2]

		svc, err := resolveService()
		if err != nil {
			return fmt.Errorf("resolve service: %w", err)
		}

		if err := svc.WriteSecret(vaultDir, key, value, secretType); err != nil {
			return fmt.Errorf("failed to write secret: %w", err)
		}

		return nil
	},
}
