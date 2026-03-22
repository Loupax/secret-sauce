package cmd

import (
	"errors"
	"fmt"
	"os"

	"filippo.io/age"
	"github.com/spf13/cobra"

	kr "github.com/loupax/secret-sauce/internal/keyring"
	vlt "github.com/loupax/secret-sauce/internal/vault"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new vault",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if vlt.Exists(vaultDir) {
			return fmt.Errorf("vault already initialized at %s; delete it first to reinitialize", vaultDir)
		}

		identity, err := age.GenerateX25519Identity()
		if err != nil {
			return fmt.Errorf("failed to generate identity: %w", err)
		}

		err = kr.Save(vaultDir, identity.String())
		if errors.Is(err, kr.ErrNoSecretService) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err != nil {
			return fmt.Errorf("failed to save key to keyring: %w", err)
		}

		err = vlt.Init(vaultDir, identity)
		if err != nil {
			return fmt.Errorf("failed to initialize vault: %w", err)
		}

		fmt.Fprintf(os.Stdout, "Vault initialized.\nPublic key (share this with teammates): %s\n", identity.Recipient())
		return nil
	},
}
