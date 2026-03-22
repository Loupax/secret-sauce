package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/loupax/secret-sauce/internal/config"
	"github.com/loupax/secret-sauce/internal/daemon"
	"github.com/loupax/secret-sauce/internal/ipc"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the secret-sauce background daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()

		timeout, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", cfg.Timeout, err)
		}

		if err := spawnDaemonProcess(timeout); err != nil {
			return fmt.Errorf("spawn daemon: %w", err)
		}

		socketPath := ipc.SocketPath()
		if waitForSocket(socketPath, 2*time.Second) {
			fmt.Println("Daemon started")
		} else {
			fmt.Fprintln(os.Stderr, "Daemon failed to start")
		}
		return nil
	},
}

// daemonServeCmd is a hidden internal subcommand — the actual daemon process runs this.
var daemonServeCmd = &cobra.Command{
	Use:    "_serve",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		timeoutStr, _ := cmd.Flags().GetString("timeout")
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", timeoutStr, err)
		}
		return daemon.NewServer(ipc.SocketPath(), timeout).Run()
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		socketPath := ipc.SocketPath()
		conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
		if err != nil {
			fmt.Println("Daemon not running")
			return nil
		}
		defer conn.Close()

		if err := json.NewEncoder(conn).Encode(ipc.Request{Op: ipc.OpShutdown}); err != nil {
			return fmt.Errorf("send shutdown: %w", err)
		}

		var resp ipc.Response
		json.NewDecoder(conn).Decode(&resp)

		fmt.Println("Daemon stopped")
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether the daemon is running",
	RunE: func(cmd *cobra.Command, args []string) error {
		socketPath := ipc.SocketPath()
		conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
		if err != nil {
			fmt.Println("Daemon is not running")
			return nil
		}
		defer conn.Close()

		if err := json.NewEncoder(conn).Encode(ipc.Request{Op: ipc.OpPing}); err != nil {
			fmt.Println("Daemon is not running")
			return nil
		}

		var resp ipc.Response
		if err := json.NewDecoder(conn).Decode(&resp); err != nil || !resp.OK {
			fmt.Println("Daemon is not running")
			return nil
		}

		fmt.Println("Daemon is running")
		return nil
	},
}

func spawnDaemonProcess(timeout time.Duration) error {
	return spawnDaemon(timeout)
}

func init() {
	daemonServeCmd.Flags().String("timeout", "15m", "idle timeout before daemon exits")

	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonServeCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
}
