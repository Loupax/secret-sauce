package cmd

import (
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
			// Print all data as key=value lines, or just "value" if only that key exists.
			if v, ok := info.Data["value"]; ok && len(info.Data) == 1 {
				fmt.Fprintln(os.Stdout, v)
			} else {
				for k, v := range info.Data {
					fmt.Fprintf(os.Stdout, "%s=%s\n", k, v)
				}
			}
			return nil
		}

		// Two-argument form: look up a specific key in the data map.
		v, ok := info.Data[args[1]]
		if !ok {
			return fmt.Errorf("key %q not found in secret %q", args[1], args[0])
		}
		fmt.Fprint(os.Stdout, v)
		return nil
	},
}

// ensure vault import is present for ErrKeyNotFound
var _ = vault.ErrKeyNotFound
