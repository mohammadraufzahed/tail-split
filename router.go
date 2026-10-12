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
	applied map[string]bool // "prefix" -> route present
}

func NewRouter() *Router {
	return &Router{applied: map[string]bool{}}
}

// defaultGateway returns the next-hop of the physical default route,
// skipping the Tailscale interface. PowerShell is used because parsing
// `route print` output is brittle.
func defaultGateway() (string, error) {
	ps := `Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue |` +
		` Where-Object { $_.NextHop -ne '0.0.0.0' -and $_.InterfaceAlias -notmatch 'Tailscale' } |` +
		` Sort-Object { $_.RouteMetric + $_.InterfaceMetric } | Select-Object -First 1 -ExpandProperty NextHop`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps).Output()
	if err != nil {
		return "", err
	}
	gw := strings.TrimSpace(string(out))
	if gw == "" {
		return "", errors.New("no physical default gateway found")
	}
	return gw, nil
}

func cidrToMask(prefixLen int) string {
	ip := net.CIDRMask(prefixLen, 32)
	return net.IP(ip).String()
}

// Apply adds routes for the given CIDR prefixes (e.g. "1.2.3.4/32").
func (r *Router) Apply(prefixes []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(prefixes) == 0 {
		return nil
	}
	if r.gateway == "" {
		gw, err := defaultGateway()
		if err != nil {
			return fmt.Errorf("default gateway: %w", err)
		}
		r.gateway = gw
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
		out, err := exec.Command("route", "add", ipnet.IP.String(), "mask", mask, r.gateway, "metric", "1").CombinedOutput()
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("route add %s: %v (%s)", p, err, strings.TrimSpace(string(out)))
			}
			continue
		}
		r.applied[p] = true
	}
	return firstErr
}

// Restore removes all routes previously added by Apply.
func (r *Router) Restore() {
	r.mu.Lock()
	defer r.mu.Unlock()
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
		_ = exec.Command("route", "delete", ipnet.IP.String(), "mask", mask, r.gateway).Run()
		delete(r.applied, p)
	}
	r.gateway = ""
}
