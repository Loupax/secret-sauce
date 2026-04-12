package ipc

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	OpListNames = "list_names"
	OpReadAll   = "read_all"
	OpWrite     = "write"
	OpDelete    = "delete"
	OpShutdown  = "shutdown"
	OpPing      = "ping"
	OpReadOne   = "read_one"
	OpGetPubKey = "get_pub_key"
)

// SecretMeta carries data for a single secret over the wire.
type SecretMeta struct {
	Data map[string]string `json:"data"`
}

type Request struct {
	Op       string            `json:"op"`
	VaultDir string            `json:"vault_dir"`
	Key      string            `json:"key,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
}

type Response struct {
	OK      bool                  `json:"ok"`
	Names   []string              `json:"names,omitempty"`
	Secrets map[string]SecretMeta `json:"secrets,omitempty"`
	Secret  *SecretMeta           `json:"secret,omitempty"`
	PubKey  string                `json:"pub_key,omitempty"`
	Error   string                `json:"error,omitempty"`
}

// SocketPath returns a safe Unix socket path for IPC.
// It prefers XDG_RUNTIME_DIR on Linux, falling back to the system temp directory.
func SocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "sauce.sock")
	}

	dir := os.TempDir()
	if runtime.GOOS == "windows" {
		// Windows temp dirs are already user-specific
		return filepath.Join(dir, "sauce.sock")
	}

	// On Unix, we include the UID to prevent collisions in shared /tmp
	return filepath.Join(dir, fmt.Sprintf("sauce-%d.sock", os.Getuid()))
}
