package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

// ExitNodeStatus reports whether Tailscale is routing through an exit node.
type ExitNodeStatus struct {
	Active   bool
	ExitNode string // Tailscale IP or hostname of the exit node, when known.
}

type tailscaleStatusJSON struct {
	ExitNodeStatus *struct {
		ID           string   `json:"ID"`
		Online       bool     `json:"Online"`
		TailscaleIPs []string `json:"TailscaleIPs"`
	} `json:"ExitNodeStatus"`
	Peer map[string]struct {
		HostName     string   `json:"HostName"`
		TailscaleIPs []string `json:"TailscaleIPs"`
		ExitNode     bool     `json:"ExitNode"`
	} `json:"Peer"`
	Self struct {
		HostName string `json:"HostName"`
	} `json:"Self"`
}

// ExitNode returns the current exit-node status by running `tailscale status --json`.
func ExitNode(ctx context.Context) ExitNodeStatus {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tailscale", "status", "--json").Output()
	if err != nil {
		return ExitNodeStatus{}
	}
	var st tailscaleStatusJSON
	if err := json.Unmarshal(out, &st); err != nil {
		return ExitNodeStatus{}
	}
	if st.ExitNodeStatus == nil || !st.ExitNodeStatus.Online {
		return ExitNodeStatus{}
	}
	name := st.ExitNodeStatus.ID
	for _, p := range st.Peer {
		if !p.ExitNode {
			continue
		}
		if p.HostName != "" {
			name = p.HostName
		}
		break
	}
	return ExitNodeStatus{Active: true, ExitNode: name}
}

// PollExitNode polls the Tailscale exit-node status and sends updates on ch.
func PollExitNode(ctx context.Context, ch chan<- ExitNodeStatus) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	last := ExitNodeStatus{}
	for {
		cur := ExitNode(ctx)
		if cur != last {
			last = cur
			select {
			case ch <- cur:
			case <-ctx.Done():
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
