package cmd

import (
	"github.com/spf13/cobra"

	"github.com/loupax/secret-sauce/internal/config"
)

var vaultDir string

var rootCmd = &cobra.Command{
	Use:   "sauce",
	Short: "A local encrypted secret vault",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if vaultDir != "" {
			return nil
		}
		
		dir, err := config.DefaultVaultDir()
		if err != nil {
			return err
		}
		vaultDir = dir
		return nil
	},
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
