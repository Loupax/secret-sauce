//go:build windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func spawnDaemon(timeout time.Duration) error {
	c := exec.Command(os.Args[0], "daemon", "_serve", "--timeout", timeout.String())
	// DETACHED_PROCESS = 0x00000008
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008}
	c.Stdin = nil
	c.Stdout = nil
	c.Stderr = nil
	return c.Start()
}
