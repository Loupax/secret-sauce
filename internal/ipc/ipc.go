package ipc

import (
	"fmt"
	"os"
)

const (
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
	Secrets map[string]SecretMeta `json:"secrets,omitempty"`
	Secret  *SecretMeta           `json:"secret,omitempty"`
	PubKey  string                `json:"pub_key,omitempty"`
	Error   string                `json:"error,omitempty"`
}

// SocketPath returns $XDG_RUNTIME_DIR/sauce.sock.
// Falls back to /tmp/sauce-<uid>.sock if XDG_RUNTIME_DIR is unset.
func SocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir + "/sauce.sock"
	}
	return fmt.Sprintf("/tmp/sauce-%d.sock", os.Getuid())
}
