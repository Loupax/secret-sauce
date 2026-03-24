package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"filippo.io/age"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	kr "github.com/loupax/secret-sauce/internal/keyring"
	vlt "github.com/loupax/secret-sauce/internal/vault"
)

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Manage vault recipients",
}

var shareAddCmd = &cobra.Command{
	Use:   "add PUBKEY",
	Short: "Add a recipient",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		unlock, err := vlt.AcquireExclusive(vaultDir)
		if err != nil {
			return fmt.Errorf("failed to acquire vault lock: %w", err)
		}
		defer unlock()

		_, err = age.ParseX25519Recipient(args[0])
		if err != nil {
			return fmt.Errorf("invalid public key %q: %w", args[0], err)
		}

		err = vlt.AppendRecipient(vaultDir, args[0])
		if err != nil {
			return fmt.Errorf("failed to append recipient: %w", err)
		}

		recipients, err := vlt.ReadRecipients(vaultDir)
		if err != nil {
			return fmt.Errorf("failed to read recipients: %w", err)
		}

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

		// Re-encrypt all existing secrets with the updated recipient list
		secrets, err := vlt.ReadAllSecrets(vaultDir, identity)
		if err != nil {
			return fmt.Errorf("failed to read secrets: %w", err)
		}

		var g errgroup.Group
		for key, info := range secrets {
			key, info := key, info
			g.Go(func() error {
				if err := vlt.WriteSecret(vaultDir, key, info.Value, info.Type, recipients, identity); err != nil {
					return fmt.Errorf("failed to re-encrypt secret %s: %w", key, err)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "Recipient %s added.\n", args[0])
		return nil
	},
}

var shareLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List recipients",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		unlock, err := vlt.AcquireShared(vaultDir)
		if err != nil {
			return fmt.Errorf("failed to acquire vault lock: %w", err)
		}
		defer unlock()

		f, err := os.Open(filepath.Join(vaultDir, ".vault_recipients"))
		if err != nil {
			return fmt.Errorf("failed to open recipients file: %w", err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				fmt.Fprintln(os.Stdout, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read recipients file: %w", err)
		}

		return nil
	},
}

func init() {
	shareCmd.AddCommand(shareAddCmd)
	shareCmd.AddCommand(shareLsCmd)
}
