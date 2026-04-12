//go:build windows

package vault

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func AcquireExclusive(vaultDir string) (func(), error) {
	return acquireLock(vaultDir, windows.LOCKFILE_EXCLUSIVE_LOCK)
}

func AcquireShared(vaultDir string) (func(), error) {
	return acquireLock(vaultDir, 0)
}

func acquireLock(vaultDir string, flags uint32) (func(), error) {
	path := filepath.Join(vaultDir, "vault.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}

	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, ol); err != nil {
		f.Close()
		return nil, err
	}

	return func() {
		windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
		f.Close()
	}, nil
}
