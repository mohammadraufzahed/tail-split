//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func isAdmin() bool {
	tok := windows.GetCurrentProcessToken()
	return tok.IsElevated()
}

// relaunchElevated restarts the current executable with the "runas" verb
// (UAC prompt) and reports whether the relaunch was issued.
func relaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Start-Process", "-FilePath", exe, "-Verb", "RunAs")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
