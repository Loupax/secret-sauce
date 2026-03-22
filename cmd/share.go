package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		matches, err := filepath.Glob(filepath.Join(vaultDir, "*.age"))
		if err != nil {
			return fmt.Errorf("failed to list secrets: %w", err)
		}

		var g errgroup.Group
		for _, match := range matches {
			match := match
			g.Go(func() error {
				base := filepath.Base(match)
				key := strings.TrimSuffix(base, ".age")

				value, err := vlt.ReadSecret(vaultDir, key, identity)
				if err != nil {
					return fmt.Errorf("failed to read secret %s: %w", key, err)
				}

				if err := vlt.WriteSecret(vaultDir, key, value, recipients); err != nil {
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
