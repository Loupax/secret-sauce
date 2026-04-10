package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/loupax/secret-sauce/internal/vault"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get <secret> [key]",
	Short: "Get a secret value",
	Args:  cobra.RangeArgs(1, 2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return completeSecretKeys(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := resolveService()
		if err != nil {
			return fmt.Errorf("resolve service: %w", err)
		}

		info, err := svc.ReadSecret(vaultDir, args[0])
		if err != nil {
			return fmt.Errorf("read secret: %w", err)
		}

		if len(args) == 1 {
			fmt.Fprintln(os.Stdout, info.Value)
			return nil
		}

		if info.Type != vault.SecretTypeMap {
			return fmt.Errorf("secret %q is not of type 'map'; cannot access key %q", args[0], args[1])
		}

		var m map[string]string
		if err := json.Unmarshal([]byte(info.Value), &m); err != nil {
			return fmt.Errorf("parse map secret: %w", err)
		}

		v, ok := m[args[1]]
		if !ok {
			return fmt.Errorf("key %q not found in secret %q", args[1], args[0])
		}
		fmt.Fprint(os.Stdout, v)
		return nil
	},
}
