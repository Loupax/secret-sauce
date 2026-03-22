package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
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

		svc, err := resolveService()
		if err != nil {
			return fmt.Errorf("resolve service: %w", err)
		}

		secrets, err := svc.ReadAllSecrets(vaultDir)
		if err != nil {
			return fmt.Errorf("read vault: %w", err)
		}

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
