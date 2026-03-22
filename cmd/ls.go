package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"filippo.io/age"
	"github.com/spf13/cobra"

	kr "github.com/loupax/secret-sauce/internal/keyring"
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

		privKey, err := kr.Load(vaultDir)
		if errors.Is(err, kr.ErrNoSecretService) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err != nil {
			return fmt.Errorf("failed to load key from keyring: %w", err)
		}

		identity, err := age.ParseX25519Identity(privKey)
		if err != nil {
			return fmt.Errorf("failed to parse identity: %w", err)
		}

		secrets, err := vlt.Read(vaultDir, identity)
		if err != nil {
			return fmt.Errorf("failed to read vault: %w", err)
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
