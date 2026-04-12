//go:build !windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func spawnDaemon(timeout time.Duration) error {
	c := exec.Command(os.Args[0], "daemon", "_serve", "--timeout", timeout.String())
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	c.Stdin = nil
	c.Stdout = nil
	c.Stderr = nil
	return c.Start()
}
