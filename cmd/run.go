package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"filippo.io/age"
	"github.com/spf13/cobra"

	kr "github.com/loupax/secret-sauce/internal/keyring"
	vlt "github.com/loupax/secret-sauce/internal/vault"
)

var runCmd = &cobra.Command{
	Use:                "run",
	Short:              "Run a command with secrets injected as env vars",
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && args[0] == "--" {
			args = args[1:]
		}
		if len(args) == 0 {
			return fmt.Errorf("usage: secret-sauce run [--] <command> [args...]")
		}

		unlock, err := vlt.AcquireShared(vaultDir)
		if err != nil {
			return fmt.Errorf("acquire shared lock: %w", err)
		}

		privKey, err := kr.Load(vaultDir)
		if err != nil {
			if errors.Is(err, kr.ErrNoSecretService) {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return fmt.Errorf("load private key: %w", err)
		}

		identity, err := age.ParseX25519Identity(privKey)
		if err != nil {
			return fmt.Errorf("parse identity: %w", err)
		}

		secrets, err := vlt.ReadAllSecrets(vaultDir, identity)
		if err != nil {
			return fmt.Errorf("read vault: %w", err)
		}

		// Release lock before exec so child processes can acquire it.
		unlock()

		combined := os.Environ()
		for k, v := range secrets {
			combined = append(combined, k+"="+v)
		}

		c := exec.Command(args[0], args[1:]...)
		c.Env = combined
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		err = c.Run()

		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			}
			// Command not found or could not be started
			fmt.Fprintln(os.Stderr, err)
			os.Exit(127)
		}
		return nil
	},
}
