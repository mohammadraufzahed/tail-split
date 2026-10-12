package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"time"
)

// ExitNodeStatus reports whether Tailscale is routing through an exit node.
type ExitNodeStatus struct {
	CLI      bool // tailscale CLI responded (false = not installed/not on PATH)
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

var tailscalePath string

// tailscaleBin locates the tailscale CLI once: PATH first, then the
// standard install dirs (an elevated process can miss user PATH edits).
func tailscaleBin() string {
	if tailscalePath != "" {
		return tailscalePath
	}
	tailscalePath = "tailscale"
	if p, err := exec.LookPath("tailscale"); err == nil {
		tailscalePath = p
		return tailscalePath
	}
	for _, c := range []string{
		`C:\Program Files\Tailscale\tailscale.exe`,
		`C:\Program Files (x86)\Tailscale\tailscale.exe`,
		`C:\Program Files\Tailscale IPN\tailscale.exe`,
	} {
		if _, err := os.Stat(c); err == nil {
			tailscalePath = c
			return tailscalePath
		}
	}
	return tailscalePath
}

// ExitNode returns the current exit-node status by running `tailscale status --json`.
func ExitNode(ctx context.Context) ExitNodeStatus {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, tailscaleBin(), "status", "--json").Output()
	if err != nil {
		return ExitNodeStatus{}
	}
	var st tailscaleStatusJSON
	if err := json.Unmarshal(out, &st); err != nil {
		return ExitNodeStatus{CLI: true}
	}
	if st.ExitNodeStatus == nil || !st.ExitNodeStatus.Online {
		return ExitNodeStatus{CLI: true}
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
	return ExitNodeStatus{CLI: true, Active: true, ExitNode: name}
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
