package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:                "gui",
	Short:              "Launch the Secret Sauce GUI",
	SilenceUsage:       true,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		bin, err := exec.LookPath("sauce-gui")
		if err != nil {
			// Fall back to looking next to the sauce binary itself.
			if self, selfErr := os.Executable(); selfErr == nil {
				candidate := filepath.Join(filepath.Dir(self), "sauce-gui")
				if _, statErr := os.Stat(candidate); statErr == nil {
					bin = candidate
				}
			}
		}
		if bin == "" {
			return fmt.Errorf("sauce-gui not found in PATH or next to sauce binary\nBuild it with: cd gui && wails build")
		}
		proc := exec.Command(bin)
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		return proc.Run()
	},
}
