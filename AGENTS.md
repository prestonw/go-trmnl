# AGENTS.md — Working Practices

## Core Principles
- Write workflows, not essays. Steps with checkable completion criteria.
- Prompt what TO do, not what not to do.
- Single source of truth. No duplication.
- Verify with fresh context — never verify your own work.
- Use strong leading words: "tight loop", "root cause", "regression test".

## go-trmnl (Go server)

### Build
- Local dev: `go build -o trmnld ./cmd/trmnld`
- Cross-compile for CT103 (ARM64): `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o trmnld ./cmd/trmnld`
- Binary: `trmnld` (~34MB ELF ARM64, statically linked)

### Deploy
- SCP to Proxmox host (white): `scp trmnld root@white:/tmp/trmnld.new`
- Push to CT103: `p -e 'pct 103 push /tmp/trmnld.new /opt/trmnl/trmnld.new --replace'`
- Swap + restart: `pct 103 exec bash -c 'cp /opt/trmnl/trmnld /opt/trmnl/trmnld.bak && mv /opt/trmnl/trmnld.new /opt/trmnl/trmnld && chmod +x /opt/trmnl/trmnld && systemctl restart trmnld'`

### Admin Auth
- Flags `-admin-user` and `-admin-password` in `/etc/systemd/system/trmnld.service`
- Read via: `p -e 'grep -E "admin-user|admin-password" /etc/systemd/system/trmnld.service'`

### Firmware OTA
- Firmware uploaded to `/opt/trmnl/data/uploads/` on CT103
- Queue OTA: `curl -X POST 'http://192.168.0.103:4567/admin/devices/{ID}/firmware' -u admin:pass --data 'firmware_url=http://192.168.0.103:4567/uploads/firmware-VERSION.bin'`
- 302 = queued successfully
- Device polls every 900s (15 min)

### Weather Plugin
- Inkplate: pure white card backgrounds, no grey/dither
- Temp range placed 10px above band
- Forecast on top-left corner, place name on top-right corner

## Server Infrastructure
- Proxmox host: white (pct CLI)
- CT103: 192.168.0.103:4567, runs trmnld + SQLite at `/opt/trmnl/data/trmnl.db`
- Devices: Inkplate 6v2 (device 3, SSF1WY, MAC ending 63:a8)

## Pushing Changes
- go-trmnl: commit → cross-compile → deploy via pct → restart service
- inkplate-trmnl: see `../inkplate-trmnl/AGENTS.md`

## Testing
- `go test ./...` runs all unit tests
- Weather rendering tests: `internal/plugins/weather/render_probe_test.go`
