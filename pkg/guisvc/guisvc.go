// Package guisvc exposes service resolution and secret types for external consumers
// (such as the GUI module) that cannot access the internal packages directly.
package guisvc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/loupax/secret-sauce/internal/ipc"
	"github.com/loupax/secret-sauce/internal/service"
	"github.com/loupax/secret-sauce/internal/vault"
)

// SecretInfo mirrors vault.SecretInfo for callers outside the module.
type SecretInfo = vault.SecretInfo

// VaultService is the strategy interface for secret vault operations.
type VaultService = service.VaultService

// Exists reports whether the vault at vaultDir has been initialized.
func Exists(vaultDir string) bool {
	return vault.Exists(vaultDir)
}

// ResolveVaultDir returns the vault directory path using the same precedence as
// the CLI: SAUCE_DIR → SECRET_SAUCE_DIR → platform data dir/secret-sauce.
// On Windows the platform default is %APPDATA%\secret-sauce.
// On Linux/macOS it follows XDG: $XDG_DATA_HOME/secret-sauce or ~/.local/share/secret-sauce.
func ResolveVaultDir() (string, error) {
	if v := os.Getenv("SAUCE_DIR"); v != "" {
		return v, nil
	}
	if v := os.Getenv("SECRET_SAUCE_DIR"); v != "" {
		return v, nil
	}
	base, err := platformDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "secret-sauce"), nil
}

func platformDataDir() (string, error) {
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			return "", fmt.Errorf("%%APPDATA%% is not set")
		}
		return appdata, nil
	}
	// Linux / macOS: respect XDG_DATA_HOME, fall back to ~/.local/share
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return xdg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share"), nil
}

// ResolveService probes the daemon socket with a ping and returns an
// IPCVaultService if the daemon is alive, otherwise a LocalVaultService.
// Unlike the CLI, this never auto-spawns the daemon.
func ResolveService() (VaultService, error) {
	socketPath := ipc.SocketPath()
	if pingSocket(socketPath) {
		return service.NewIPCVaultService(socketPath), nil
	}
	return service.NewLocalVaultService(), nil
}

// pingSocket dials the Unix socket and sends an OpPing request, mirroring
// the probe logic in cmd/service_resolver.go.
func pingSocket(path string) bool {
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
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
