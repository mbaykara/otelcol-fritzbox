# continue-here

## Project
fritzotel-receiver — OTel Collector **metrics receiver** for AVM Fritz!Box
routers (TR-064 API). Spec: `docs/superpowers/specs/2026-09-07-fritzbox-receiver-design.md`.

## Current state (2026-09-07 ~09:10)
**DONE and verified live. Tagged `v0.1.0`.**

- Full receiver: `receiver/fritzbox/` (config, factory, scraper, mdatagen).
- TR-064 client: `internal/tr064/` — SOAP + RFC 7616 digest with
  **nc-progression** (Fritz!Box enforces replay protection) and fault parsing
  that handles HTTP-500-wrapped SOAP faults.
- All unit tests green; vet/gofmt clean on our code.
- OCB example builds and runs against the live box (Fritz!OS 8.25, VDSL):
  15 metrics, 40 datapoints, 0 scrape errors.
- Password file `/var/folders/.../T/opencode/fritzbox_pw` deleted.
- Git: 8 commits + tag `v0.1.0` on `main`, no remote configured.

## Live verification results
- DSL rates are **kbit/s from TR-064 → converted to bit/s** (UCUM) in the
  receiver. Verified against Fritz UI values.
- WLAN mapping confirmed: wlan1=2.4GHz, wlan2=5GHz, wlan3=guest (SSID attr
  distinguishes them).
- WANIPConnection service faults on this box (UPnP 401/502) → group is
  skipped with warn-once, not a scrape failure. `fritzbox.wan.connection.*`
  and `fritzbox.wan.external_ip` are therefore absent on this box; they work
  on boxes where the service is functional.

## Known limitations / next ideas
- `hw.network.bandwidth.utilization` skipped: the AVM action
  `X_AVM-DE_GetCommonLinkProperties` requires a permission scope this user
  lacks (UPnP 401).
- `fritzbox.hosts.active` iterates N hosts (one SOAP call each) — off by
  default, enabled in example config.
- No remote yet: `git remote add origin ...` when publishing.

## Usage
```sh
cd example
go tool go.opentelemetry.io/collector/cmd/builder --config builder-config.yaml
FRITZBOX_USERNAME=... FRITZBOX_PASSWORD=... \
  ./otelcol-fritzbox/otelcol-fritzbox --config collector-config.yaml
```
