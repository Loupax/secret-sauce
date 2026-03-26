package cmd

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/loupax/sauce/internal/config"
	"github.com/loupax/sauce/internal/ipc"
	"github.com/loupax/sauce/internal/service"
)

// resolveService implements the Hybrid Execution Decision Tree:
//
//  1. Probe the Unix socket with a ping.
//  2. If alive → return IPCVaultService.
//  3. If dead AND auto_spawn=true → clean socket, spawn daemon, wait, return IPCVaultService.
//  4. If dead AND auto_spawn=false → return LocalVaultService.
func resolveService() (service.VaultService, error) {
	cfg, _ := config.Load() // ignore error, use defaults
	socketPath := ipc.SocketPath()

	if isSocketAlive(socketPath) {
		return service.NewIPCVaultService(socketPath), nil
	}

	if cfg.AutoSpawn {
		timeout, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			timeout = 15 * time.Minute
		}
		if err := spawnDaemon(timeout); err == nil && waitForSocket(socketPath, 2*time.Second) {
			return service.NewIPCVaultService(socketPath), nil
		}
	}

	return service.NewLocalVaultService(), nil
}

func isSocketAlive(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(ipc.Request{Op: ipc.OpPing}); err != nil {
		return false
	}

	var resp ipc.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return false
	}
	return resp.OK
}

func spawnDaemon(timeout time.Duration) error {
	c := exec.Command(os.Args[0], "daemon", "_serve", "--timeout", timeout.String())
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	c.Stdin = nil
	c.Stdout = nil
	c.Stderr = nil
	return c.Start()
}

func waitForSocket(socketPath string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
