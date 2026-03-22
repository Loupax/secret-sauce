package vault

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func acquireLock(vaultDir string, how int) (func(), error) {
	path := filepath.Join(vaultDir, "vault.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return func() {}, err
	}
	if err := unix.Flock(int(f.Fd()), how); err != nil {
		f.Close()
		return func() {}, fmt.Errorf("flock: %w", err)
	}
	unlock := func() {
		unix.Flock(int(f.Fd()), unix.LOCK_UN)
		f.Close()
	}
	return unlock, nil
}

func AcquireExclusive(vaultDir string) (func(), error) {
	return acquireLock(vaultDir, unix.LOCK_EX)
}

func AcquireShared(vaultDir string) (func(), error) {
	return acquireLock(vaultDir, unix.LOCK_SH)
}
