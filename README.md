# tail-split

Split-tunneling for a Tailscale **exit node** on Windows: keep chosen IPs,
subnets, or whole domains on your normal internet connection while
everything else goes through the exit node.

Tailscale itself can only route *everything* or *LAN* (`--exit-node-allow-lan-access`)
around the exit node — it has no per-destination exclusions on Windows.
tail-split fills that gap: it watches `tailscale status`, and while an exit
node is active it installs low-overhead `route add` entries that send your
excluded destinations via the physical default gateway instead.

## How it works

- Watches `tailscale status --json` every 5 s; exclusions apply only while
  an exit node is connected and are removed when it disconnects, when the
  app is toggled off, or on quit.
- Entries: IPv4 (`1.2.3.4`), CIDR (`10.0.0.0/8`), or domain (`example.com`).
  Domains are resolved to their current A records (each becomes a /32) and
  re-resolved every 60 s — note that domains on fast-rotating CDN IPs may
  need a wider CIDR instead.
- Routes are added for the specific prefixes via the physical gateway.
  Tailscale captures traffic with `0.0.0.0/1` + `128.0.0.0/1`, so any more
  specific prefix wins automatically — no metric fight needed.
- Managed routes are removed on clean exit. If the process is killed, they
  are non-persistent `route add` entries — a reboot (or route delete)
  clears them.

## Tray menu

- **Exit node: …** — current status (read-only).
- **Enabled** — master on/off for all exclusions.
- **Add exclusion…** — small dialog for an IP / CIDR / domain.
- **Exclusions ▸** — per-entry checkmark (enable/disable) and `Remove`.
- **Open config…** — opens `%APPDATA%\tail-split\config.json`.
- **Quit** — restores routes and exits.

The app asks for administrator elevation on start (route changes require it).

## Download

Grab `tail-split.exe` from the latest
[GitHub Release](https://github.com/mohammadraufzahed/tail-split/releases).
Releases are built automatically on `v*` tags (windows/amd64, GUI
subsystem, exe icon embedded).

## Build

Windows:

```powershell
go build -ldflags="-H windowsgui" -o tail-split.exe .
```

To also embed the exe icon (optional):

```powershell
go install github.com/akavel/rsrc@latest
rsrc -ico icon.ico -o tail-split.syso
go build -ldflags="-H windowsgui" -o tail-split.exe .
```

Linux/macOS: not supported (route management is Windows-specific).

## Limitations

- IPv4 only.
- Domain exclusions track DNS, not SNI — a domain that resolves to an IP
  shared with other sites excludes that whole IP.
- No wildcard domains (`*.example.com`) — add the exact hosts you use.
