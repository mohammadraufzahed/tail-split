//go:build windows

package main

import (
	"os/exec"
	"strings"
)

// Interfaces lists alias names of interfaces that have a non-Tailscale
// IPv4 default route — the candidates for carrying excluded traffic.
func Interfaces() []string {
	ps := `Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue |` +
		` Where-Object { $_.NextHop -ne '0.0.0.0' -and $_.InterfaceAlias -notmatch 'Tailscale' } |` +
		` Select-Object -ExpandProperty InterfaceAlias -Unique`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps).Output()
	if err != nil {
		return nil
	}
	var res []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			res = append(res, line)
		}
	}
	return res
}
