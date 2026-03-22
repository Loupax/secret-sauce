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

		secrets[args[0]] = args[1]

		recipients, err := vlt.ReadRecipients(vaultDir)
		if err != nil {
			return fmt.Errorf("failed to read recipients: %w", err)
		}

		err = vlt.Write(vaultDir, secrets, recipients)
		if err != nil {
			return fmt.Errorf("failed to write vault: %w", err)
		}

		return nil
	},
}
