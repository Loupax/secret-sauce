package ipc

import (
	"fmt"
	"os"
)

const (
	OpReadAll  = "read_all"
	OpWrite    = "write"
	OpDelete   = "delete"
	OpShutdown = "shutdown"
	OpPing     = "ping"
)

type Request struct {
	Op       string `json:"op"`
	VaultDir string `json:"vault_dir"`
	Key      string `json:"key,omitempty"`
	Value    string `json:"value,omitempty"`
}

type Response struct {
	OK      bool              `json:"ok"`
	Secrets map[string]string `json:"secrets,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// SocketPath returns $XDG_RUNTIME_DIR/secret-sauce.sock.
// Falls back to /tmp/secret-sauce-<uid>.sock if XDG_RUNTIME_DIR is unset.
func SocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir + "/secret-sauce.sock"
	}
	return fmt.Sprintf("/tmp/secret-sauce-%d.sock", os.Getuid())
}
