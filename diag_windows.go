//go:build windows

package main

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// srcAddr returns the primary IPv4 address of the interface with the
// given index — used as ping -S source to force the physical NIC.
func srcAddr(ifIndex string) (string, error) {
	ps := `Get-NetIPAddress -InterfaceIndex ` + ifIndex +
		` -AddressFamily IPv4 -ErrorAction SilentlyContinue |` +
		` Select-Object -First 1 -ExpandProperty IPAddress`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RunCheck pings each applied prefix through the physical interface and
// writes findings to the log. Returns a one-line summary for the menu.
func (r *Router) RunCheck() string {
	r.mu.Lock()
	ifIndex := r.ifIndex
	gw := r.gateway
	var prefixes []string
	for p := range r.applied {
		prefixes = append(prefixes, p)
	}
	r.mu.Unlock()

	if gw == "" {
		logf("check: no gateway detected yet")
		return "Check: no routes applied"
	}
	src, err := srcAddr(ifIndex)
	logf("check: gateway=%s if=%s src=%s routes=%d", gw, ifIndex, src, len(prefixes))
	if err != nil || src == "" {
		return "Check: can't read interface address"
	}
	failed := 0
	for _, p := range prefixes {
		host, _, err := net.ParseCIDR(p)
		if err != nil {
			host = net.ParseIP(strings.TrimSuffix(p, "/32"))
		}
		if host == nil {
			continue
		}
		out, err := exec.Command("ping", "-n", "2", "-w", "2000", "-S", src, host.String()).CombinedOutput()
		if err != nil {
			failed++
			logf("check: %s via %s FAIL: %s", host, src, oneLine(string(out)))
		} else {
			logf("check: %s via %s OK", host, src)
		}
	}
	if failed > 0 {
		return fmt.Sprintf("Check: %d/%d failed — see log", failed, len(prefixes))
	}
	return fmt.Sprintf("Check: %d routes OK", len(prefixes))
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	parts := strings.Split(s, "\n")
	var last string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			last = p
		}
	}
	return last
}
