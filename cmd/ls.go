package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List secret keys",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := resolveService()
		if err != nil {
return fmt.Errorf("failed to initialize vault service: %w", err)
		}

		secrets, err := svc.ReadAllSecrets(vaultDir)
		if err != nil {
			return fmt.Errorf("failed to list secrets: %w", err)
		}

		keys := make([]string, 0, len(secrets))
		for k := range secrets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintln(os.Stdout, key)
		}

		return nil
	},
}
