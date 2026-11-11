//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// startDetached starts woolly in its own session, independent of this
// terminal. Only Windows uses it today; launchd and systemd start the
// service themselves.
func startDetached(exe string, args []string) error {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
