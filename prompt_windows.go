//go:build windows

package main

import (
	"os/exec"
	"strings"
)

// promptInput shows a small input dialog and returns the entered text.
// Uses a VBScript InputBox via PowerShell so no extra UI deps are needed.
func promptInput(title, message string) (string, error) {
	ps := `Add-Type -AssemblyName Microsoft.VisualBasic;` +
		`[Microsoft.VisualBasic.Interaction]::InputBox("` + strings.ReplaceAll(message, `"`, `''`) +
		`","` + strings.ReplaceAll(title, `"`, `''`) + `","")`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
