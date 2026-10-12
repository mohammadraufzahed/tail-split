package main

import (
	"context"
	_ "embed"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var iconData []byte

// buildVersion is stamped by the release workflow (-X main.buildVersion=vX.Y.Z).
var buildVersion = "dev"

const reResolveInterval = 60 * time.Second

type app struct {
	store  *Store
	router *Router

	mu         sync.Mutex
	exitActive bool
	exitName   string

	mStatus    *systray.MenuItem
	mRoutes    *systray.MenuItem
	mCheck     *systray.MenuItem
	mToggle    *systray.MenuItem
	mEntries   *systray.MenuItem
	mIface     *systray.MenuItem
	entryItems []*systray.MenuItem
	ifaceItems []*systray.MenuItem
}

func main() {
	initLog()
	logf("tail-split %s starting", buildVersion)
	if !isAdmin() {
		if err := relaunchElevated(); err == nil {
			return // elevated instance takes over
		}
		// fall through: run anyway, route ops will just fail
	}
	systray.Run(onReady, onExit)
}

var a *app

func onReady() {
	a = &app{store: NewStore(), router: NewRouter()}
	a.store.Load()

	systray.SetIcon(iconData)
	systray.SetTooltip("tail-split " + buildVersion + " — exit node exclusions")

	a.mStatus = systray.AddMenuItem("Exit node: detecting…", "Current Tailscale exit node status")
	a.mStatus.Disable()
	a.mRoutes = systray.AddMenuItem("Routes: 0 active", "Exclusion routes currently installed")
	a.mRoutes.Disable()
	systray.AddSeparator()
	a.mToggle = systray.AddMenuItemCheckbox("Enabled", "Enable/disable all exclusions", a.store.Config().Enabled)
	mAdd := systray.AddMenuItem("Add exclusion…", "Exclude an IP, CIDR, or domain")
	a.mEntries = systray.AddMenuItem("Exclusions", "Destinations kept off the exit node")
	a.mIface = systray.AddMenuItem("Interface", "Interface that carries excluded traffic")
	systray.AddSeparator()
	mDiag := systray.AddMenuItem("Diagnostics", "Check routes and view the log")
	mCheck := mDiag.AddSubMenuItem("Run check", "Ping excluded destinations via the physical interface")
	mOpenLog := mDiag.AddSubMenuItem("Open log", "Open tail-split.log")
	a.mCheck = mDiag.AddSubMenuItem("", "")
	a.mCheck.Hide()
	mEdit := systray.AddMenuItem("Open config…", "Edit config.json in your editor")
	mQuit := systray.AddMenuItem("Quit", "Remove managed routes and exit")

	ctx, cancel := context.WithCancel(context.Background())
	statusCh := make(chan ExitNodeStatus, 1)
	go PollExitNode(ctx, statusCh)

	a.rebuildEntries()
	a.rebuildIfaces()
	go a.eventLoop(cancel, statusCh, mAdd.ClickedCh, mEdit.ClickedCh, mQuit.ClickedCh,
		mCheck.ClickedCh, mOpenLog.ClickedCh)
}

func onExit() {
	if a != nil {
		a.router.Restore()
	}
}

// rebuildEntries refreshes the Exclusions submenu from config.
func (a *app) rebuildEntries() {
	for _, it := range a.entryItems {
		it.Hide()
	}
	a.entryItems = nil

	cfg := a.store.Config()
	if len(cfg.Entries) == 0 {
		e := a.mEntries.AddSubMenuItem("No exclusions yet", "")
		e.Disable()
		a.entryItems = append(a.entryItems, e)
		return
	}
	for _, e := range cfg.Entries {
		entry := e
		sub := a.mEntries.AddSubMenuItemCheckbox(entry.Value, "", entry.Enabled)
		a.entryItems = append(a.entryItems, sub)
		go func(v string, ch <-chan struct{}) {
			for range ch {
				a.store.ToggleEntry(v)
				a.refreshNow()
				a.rebuildEntries()
			}
		}(entry.Value, sub.ClickedCh)
		rm := sub.AddSubMenuItem("Remove", "")
		a.entryItems = append(a.entryItems, rm)
		go func(v string, ch <-chan struct{}) {
			for range ch {
				a.store.RemoveEntry(v)
				a.refreshNow()
				a.rebuildEntries()
			}
		}(entry.Value, rm.ClickedCh)
	}
}

// rebuildIfaces refreshes the Interface submenu: Auto-detect plus every
// interface that currently has a non-Tailscale default route.
func (a *app) rebuildIfaces() {
	for _, it := range a.ifaceItems {
		it.Hide()
	}
	a.ifaceItems = nil

	sel := a.store.Config().Interface
	auto := a.mIface.AddSubMenuItemCheckbox("Auto-detect", "", sel == "")
	a.ifaceItems = append(a.ifaceItems, auto)
	go func(ch <-chan struct{}) {
		for range ch {
			a.store.SetInterface("")
			a.refreshNow()
			a.rebuildIfaces()
		}
	}(auto.ClickedCh)

	for _, alias := range Interfaces() {
		al := alias
		it := a.mIface.AddSubMenuItemCheckbox(al, "", sel == al)
		a.ifaceItems = append(a.ifaceItems, it)
		go func(ch <-chan struct{}) {
			for range ch {
				a.store.SetInterface(al)
				a.refreshNow()
				a.rebuildIfaces()
			}
		}(it.ClickedCh)
	}
}

// refreshNow re-resolves and applies/removes routes immediately.
func (a *app) refreshNow() {
	a.mu.Lock()
	active := a.exitActive
	a.mu.Unlock()
	cfg := a.store.Config()
	if active && cfg.Enabled {
		prefixes := Resolve(context.Background(), cfg.Entries)
		logf("apply: %d entries -> %d prefixes %v", len(cfg.Entries), len(prefixes), prefixes)
		if err := a.router.Apply(prefixes, cfg.Interface); err != nil {
			logf("apply error: %v", err)
		}
	} else {
		a.router.Restore()
	}
	a.mRoutes.SetTitle(fmt.Sprintf("Routes: %d active", a.router.Count()))
}

func (a *app) eventLoop(cancel context.CancelFunc, statusCh <-chan ExitNodeStatus, addCh, editCh, quitCh, checkCh, openLogCh <-chan struct{}) {
	reResolve := time.NewTicker(reResolveInterval)
	defer reResolve.Stop()
	for {
		select {
		case st := <-statusCh:
			a.mu.Lock()
			a.exitActive = st.Active
			a.exitName = st.ExitNode
			a.mu.Unlock()
			if !st.CLI {
				a.mStatus.SetTitle("Tailscale CLI not found")
				logf("tailscale CLI not responding — is it installed and on PATH?")
			} else if st.Active {
				a.mStatus.SetTitle("Exit node: " + st.ExitNode)
				logf("exit node connected: %s", st.ExitNode)
			} else {
				a.mStatus.SetTitle("Exit node: not connected")
				logf("exit node disconnected")
			}
			a.refreshNow()
		case <-reResolve.C:
			a.mu.Lock()
			active := a.exitActive
			a.mu.Unlock()
			if active && a.store.Config().Enabled {
				a.refreshNow()
			}
		case <-a.mToggle.ClickedCh:
			cfg := a.store.Config()
			a.store.SetEnabled(!cfg.Enabled)
			if !cfg.Enabled {
				a.mToggle.Check()
			} else {
				a.mToggle.Uncheck()
			}
			a.refreshNow()
		case <-addCh:
			v, err := promptInput("tail-split", "IP, CIDR, or domain to exclude (e.g. 1.2.3.4, 10.0.0.0/8, example.com)")
			if err == nil && v != "" {
				if a.store.AddEntry(v) {
					a.rebuildEntries()
				}
				a.refreshNow()
			}
		case <-checkCh:
			go func() {
				res := a.router.RunCheck()
				a.mCheck.SetTitle(res)
				a.mCheck.Show()
			}()
		case <-openLogCh:
			_ = exec.Command("cmd", "/c", "start", "", logPath).Start()
		case <-editCh:
			_ = exec.Command("cmd", "/c", "start", "", a.store.Path()).Start()
		case <-quitCh:
			cancel()
			systray.Quit()
			return
		}
	}
}
