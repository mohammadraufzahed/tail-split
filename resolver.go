package main

import (
	"context"
	"net"
	"strings"
	"time"
)

// Resolve maps config entries to IPv4 CIDR prefixes.
// IPs and CIDRs pass through; hostnames are resolved and each A record
// becomes a /32.
func Resolve(ctx context.Context, entries []Entry) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	r := net.DefaultResolver
	for _, e := range entries {
		if !e.Enabled {
			continue
		}
		v := strings.TrimSpace(e.Value)
		if v == "" {
			continue
		}
		if ip := net.ParseIP(v); ip != nil && ip.To4() != nil {
			add(ip.String() + "/32")
			continue
		}
		if _, ipnet, err := net.ParseCIDR(v); err == nil {
			if ipnet.IP.To4() != nil {
				add(ipnet.String())
			}
			continue
		}
		// treat as hostname
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		ips, err := r.LookupIP(ctx, "ip4", v)
		cancel()
		if err != nil {
			continue
		}
		for _, ip := range ips {
			if ip4 := ip.To4(); ip4 != nil {
				add(ip4.String() + "/32")
			}
		}
	}
	return out
}
