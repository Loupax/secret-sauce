package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/loupax/secret-sauce/internal/vault"
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
			return fmt.Errorf("usage: sauce run [--] <command> [args...]")
		}

		svc, err := resolveService()
		if err != nil {
			return fmt.Errorf("resolve service: %w", err)
		}

		secrets, err := svc.ReadAllSecrets(vaultDir)
		if err != nil {
			return fmt.Errorf("read vault: %w", err)
		}

		var extraFiles []*os.File
		defer func() {
			for _, f := range extraFiles {
				f.Close()
			}
		}()

		combined := os.Environ()
		for k, info := range secrets {
			switch info.Type {
			case vault.SecretTypeEnvironment:
				combined = append(combined, k+"="+info.Value)

			case vault.SecretTypeFile:
				tmpFile, err := os.CreateTemp("", "secret-sauce-*")
				if err != nil {
					return fmt.Errorf("create temp file for secret %q: %w", k, err)
				}
				extraFiles = append(extraFiles, tmpFile)

				if err := os.Remove(tmpFile.Name()); err != nil {
					return fmt.Errorf("unlink temp file for secret %q: %w", k, err)
				}

				if _, err := fmt.Fprint(tmpFile, info.Value); err != nil {
					return fmt.Errorf("write temp file for secret %q: %w", k, err)
				}

				if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
					return fmt.Errorf("seek temp file for secret %q: %w", k, err)
				}

				fdIndex := 3 + len(extraFiles) - 1
				combined = append(combined, fmt.Sprintf("%s=/dev/fd/%d", k, fdIndex))
			}
		}

		c := exec.Command(args[0], args[1:]...)
		c.Env = combined
		c.ExtraFiles = extraFiles
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
