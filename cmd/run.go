package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/loupax/secret-sauce/internal/manifest"
	"github.com/pelletier/go-toml/v2"
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

		// Parse sauce.toml from the working directory.
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		manifestPath := filepath.Join(wd, "sauce.toml")
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("sauce.toml not found; create it manually to wire secrets")
			}
			return fmt.Errorf("read sauce.toml: %w", err)
		}
		var mf manifest.Manifest
		if err := toml.Unmarshal(manifestData, &mf); err != nil {
			return fmt.Errorf("parse sauce.toml: %w", err)
		}

		svc, err := resolveService()
		if err != nil {
			return fmt.Errorf("resolve service: %w", err)
		}

		// Collect unique secret names referenced by the manifest.
		// RHS format: "secret_name" (uses Data["value"]) or "secret_name.field" (uses Data["field"]).
		nameSet := make(map[string]struct{})
		for _, ref := range mf.Env {
			nameSet[secretRef(ref).name()] = struct{}{}
		}
		for _, ref := range mf.File {
			nameSet[secretRef(ref).name()] = struct{}{}
		}

		// TODO: fetch secrets concurrently (errgroup) if local-path latency
		// becomes measurable in practice. Daemon path already uses a concurrent
		// index so this only affects no-daemon runs with large sauce.toml files.
		// Fetch each unique secret once.
		fetched := make(map[string]map[string]string, len(nameSet))
		for name := range nameSet {
			info, err := svc.ReadSecret(vaultDir, name)
			if err != nil {
				return fmt.Errorf("read secret %q: %w", name, err)
			}
			fetched[name] = info.Data
		}

		var extraFiles []*os.File
		defer func() {
			for _, f := range extraFiles {
				f.Close()
			}
		}()

		combined := os.Environ()

		// Inject env secrets.
		for envVar, ref := range mf.Env {
			r := secretRef(ref)
			d, ok := fetched[r.name()]
			if !ok {
				return fmt.Errorf("secret %q not found for env var %q", r.name(), envVar)
			}
			val, ok := d[r.field()]
			if !ok {
				return fmt.Errorf("secret %q has no field %q", r.name(), r.field())
			}
			combined = append(combined, envVar+"="+val)
		}

		// Inject file secrets.
		for envVar, ref := range mf.File {
			r := secretRef(ref)
			d, ok := fetched[r.name()]
			if !ok {
				return fmt.Errorf("secret %q not found for file var %q", r.name(), envVar)
			}
			val, ok := d[r.field()]
			if !ok {
				return fmt.Errorf("secret %q has no field %q", r.name(), r.field())
			}

			tmpFile, err := os.CreateTemp("", "secret-sauce-*")
			if err != nil {
				return fmt.Errorf("create temp file for secret %q: %w", r.name(), err)
			}
			extraFiles = append(extraFiles, tmpFile)

			if err := os.Remove(tmpFile.Name()); err != nil {
				return fmt.Errorf("unlink temp file for secret %q: %w", r.name(), err)
			}

			if _, err := fmt.Fprint(tmpFile, val); err != nil {
				return fmt.Errorf("write temp file for secret %q: %w", r.name(), err)
			}

			if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("seek temp file for secret %q: %w", r.name(), err)
			}

			fdIndex := 3 + len(extraFiles) - 1
			combined = append(combined, fmt.Sprintf("%s=/dev/fd/%d", envVar, fdIndex))
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

// secretRef parses a sauce.toml RHS value.
// "secret_name"       → name=secret_name, field=value
// "secret_name.field" → name=secret_name, field=field
type secretRef string

func (r secretRef) name() string {
	s := string(r)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return s[:i]
	}
	return s
}

func (r secretRef) field() string {
	s := string(r)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return s[i+1:]
	}
	return "value"
}
