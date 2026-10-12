//go:build windows

package main

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"sort"
	"strings"
	"sync"
)

// Router manages Windows routes that send selected destinations via the
// physical default gateway instead of the Tailscale exit node.
type Router struct {
	mu      sync.Mutex
	gateway string
	ifIndex string
	ifAlias string          // selected interface, "" = auto
	applied map[string]bool // "prefix" -> route present
}

func NewRouter() *Router {
	return &Router{applied: map[string]bool{}}
}

// defaultGateway returns the next-hop and interface index of the physical
// default route, skipping the Tailscale interface. When ifAlias is set,
// only that interface is considered. PowerShell is used because parsing
// `route print` output is brittle.
func defaultGateway(ifAlias string) (gw string, ifIndex string, err error) {
	filter := `$_.InterfaceAlias -notmatch 'Tailscale'`
	if ifAlias != "" {
		filter = `$_.InterfaceAlias -eq '` + strings.ReplaceAll(ifAlias, "'", "''") + `'`
	}
	ps := `$r = Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue |` +
		` Where-Object { $_.NextHop -ne '0.0.0.0' -and ` + filter + ` } |` +
		` Sort-Object { $_.RouteMetric + $_.InterfaceMetric } | Select-Object -First 1;` +
		` if ($r) { "$($r.NextHop) $($r.InterfaceIndex)" }`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps).Output()
	if err != nil {
		return "", "", err
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		return "", "", errors.New("no physical default gateway found")
	}
	return fields[0], fields[1], nil
}

// Apply adds routes for the given CIDR prefixes (e.g. "1.2.3.4/32").
// ifAlias selects the carrying interface ("" = auto-detect); changing it
// re-detects the gateway and re-pins all routes.
func (r *Router) Apply(prefixes []string, ifAlias string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ifAlias != r.ifAlias {
		r.restoreLocked()
		r.ifAlias = ifAlias
	}
	if len(prefixes) == 0 {
		return nil
	}
	if r.gateway == "" {
		gw, ifi, err := defaultGateway(ifAlias)
		if err != nil {
			logf("gateway detect failed: %v", err)
			return fmt.Errorf("default gateway: %w", err)
		}
		r.gateway = gw
		r.ifIndex = ifi
		logf("gateway detected: %s via interface %s (alias %q)", gw, ifi, ifAlias)
	}
	var firstErr error
	for _, p := range prefixes {
		if r.applied[p] {
			continue
		}
		_, ipnet, err := net.ParseCIDR(p)
		if err != nil {
			// bare IP -> /32
			ip := net.ParseIP(p)
			if ip == nil || ip.To4() == nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("bad prefix %q", p)
				}
				continue
			}
			ipnet = &net.IPNet{IP: ip.To4(), Mask: net.CIDRMask(32, 32)}
			p = ipnet.String()
		}
		mask := net.IP(ipnet.Mask).String()
		// IF pins the route to the physical interface — without it Windows
		// can bind the route to the Tailscale interface and the excluded
		// traffic dies inside the tunnel.
		out, err := exec.Command("route", "add", ipnet.IP.String(), "mask", mask,
			r.gateway, "metric", "1", "IF", r.ifIndex).CombinedOutput()
		if err != nil {
			logf("route add %s via %s IF %s failed: %v (%s) — retrying without IF",
				p, r.gateway, r.ifIndex, err, strings.TrimSpace(string(out)))
			// fallback: let Windows pick the interface from the gateway
			out, err = exec.Command("route", "add", ipnet.IP.String(), "mask", mask,
				r.gateway, "metric", "1").CombinedOutput()
		}
		if err != nil {
			logf("route add %s via %s FAILED: %v (%s)",
				p, r.gateway, err, strings.TrimSpace(string(out)))
			if firstErr == nil {
				firstErr = fmt.Errorf("route add %s: %v (%s)", p, err, strings.TrimSpace(string(out)))
			}
			continue
		}
		logf("route add %s via %s IF %s ok", p, r.gateway, r.ifIndex)
		r.applied[p] = true
	}
	return firstErr
}

// Count reports how many exclusion routes are currently installed.
func (r *Router) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.applied)
}

// Restore removes all routes previously added by Apply.
func (r *Router) Restore() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.restoreLocked()
	r.ifAlias = ""
}

func (r *Router) restoreLocked() {
	var keys []string
	for p := range r.applied {
		keys = append(keys, p)
	}
	// longest prefixes first (order is cosmetic)
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, p := range keys {
		_, ipnet, err := net.ParseCIDR(p)
		if err != nil {
			continue
		}
		mask := net.IP(ipnet.Mask).String()
		out, err := exec.Command("route", "delete", ipnet.IP.String(), "mask", mask, r.gateway).CombinedOutput()
		if err != nil {
			logf("route delete %s FAILED: %v (%s)", p, err, strings.TrimSpace(string(out)))
		} else {
			logf("route delete %s ok", p)
		}
		delete(r.applied, p)
	}
	r.gateway = ""
	r.ifIndex = ""
}
