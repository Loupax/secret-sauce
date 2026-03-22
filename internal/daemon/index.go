package daemon

import (
	"sync"
	"time"

	"github.com/loupax/secret-sauce/internal/vault"
)

// IndexEntry holds a decrypted secret and the UUID filename that contains it.
type IndexEntry struct {
	UUID     string               // UUID only, no .age suffix
	Envelope vault.SecretEnvelope
}

// VaultIndex is the daemon's in-memory cache of all decrypted secrets.
type VaultIndex struct {
	mu         sync.RWMutex
	entries    map[string]IndexEntry // name → entry
	dirModTime time.Time
}

func newVaultIndex() *VaultIndex {
	return &VaultIndex{entries: make(map[string]IndexEntry)}
}
