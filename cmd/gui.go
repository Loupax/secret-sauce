package cmd

import (
	"fmt"
	"os"
	"os/exec"

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
			return fmt.Errorf("sauce-gui not found in PATH\nBuild it with: cd gui && wails build")
		}
		proc := exec.Command(bin)
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		return proc.Run()
	},
}
