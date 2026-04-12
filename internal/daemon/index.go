package daemon

import (
	"sync"
	"time"
)

// IndexEntry maps a secret name to its UUID filename on disk.
// Values are intentionally NOT cached — decrypted on demand per request.
type IndexEntry struct {
	UUID string // UUID only, no .age suffix
}

// VaultIndex is the daemon's in-memory name→UUID mapping.
type VaultIndex struct {
	mu         sync.RWMutex
	entries    map[string]IndexEntry // name → UUID
	dirModTime time.Time
}

func newVaultIndex() *VaultIndex {
	return &VaultIndex{entries: make(map[string]IndexEntry)}
}
