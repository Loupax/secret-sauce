package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var vaultDir string

var rootCmd = &cobra.Command{
	Use:   "sauce",
	Short: "A local encrypted secret vault",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if vaultDir != "" {
			return nil
		}
		if v := os.Getenv("SAUCE_DIR"); v != "" {
			vaultDir = v
			return nil
		}
		if v := os.Getenv("SECRET_SAUCE_DIR"); v != "" {
			vaultDir = v
			return nil
		}
		home, err := platformDataDir()
		if err != nil {
			return err
		}
		vaultDir = filepath.Join(home, "secret-sauce")
		return nil
	},
}

func platformDataDir() (string, error) {
	if runtime.GOOS == "windows" {
		if v := os.Getenv("APPDATA"); v != "" {
			return v, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("%%APPDATA%% is not set and home directory could not be determined: %w", err)
		}
		return filepath.Join(home, "AppData", "Roaming"), nil
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&vaultDir, "vault-dir", "", "path to vault directory")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(shareCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(guiCmd)
}
