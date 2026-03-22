package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	vlt "github.com/loupax/secret-sauce/internal/vault"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List secret keys",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		unlock, err := vlt.AcquireShared(vaultDir)
		if err != nil {
			return fmt.Errorf("failed to acquire vault lock: %w", err)
		}
		defer unlock()

		matches, err := filepath.Glob(filepath.Join(vaultDir, "*.age"))
		if err != nil {
			return fmt.Errorf("failed to list secrets: %w", err)
		}

		keys := make([]string, 0, len(matches))
		for _, match := range matches {
			name := filepath.Base(match)
			key := strings.TrimSuffix(name, ".age")
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintln(os.Stdout, key)
		}

		return nil
	},
}
