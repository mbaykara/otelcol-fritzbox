# continue-here

## Project
fritzotel-receiver — custom OTel Collector **metrics receiver** for AVM
Fritz!Box routers (TR-064 API), importable into any collector via OCB.

## Current state (as of 2026-09-07)
- Greenfield repo, no code yet.
- Design **approved** by user; spec at
  `docs/superpowers/specs/2026-09-07-fritzbox-receiver-design.md`.
- Next step: implementation (writing-plans skill was the planned next move in
  the brainstorming flow).

## Key decisions
- Approach: contrib-style scraper = `mdatagen` + `scraperhelper` + own
  `internal/tr064` SOAP client (HTTP digest auth, RFC 7616 MD5).
- Module path: `github.com/mbaykara/fritzotel-receiver` (flat layout, receiver
  at repo root).
- Metric contract = MIXED: released `hw.network.*` semconv (io, packets,
  bandwidth.limit, up, errors w/ `error.type=fec/crc/hec`,
  bandwidth.utilization) + custom `fritzbox.*` (device/wan/dsl/wlan/hosts).
- `network.io.direction` = receive/transmit (NOT up/down). `hw.id` synthesized
  (fritzbox.wan, fritzbox.wlan1..3).
- Scope: TR-064 only, no Lua API. Metrics only.
- `mdatagen` + `builder` pinned as Go tool deps (Go 1.27 present).

## Live target
- Fritz!Box reachable at http://192.168.178.1:49000 (fritz.box), Fritz!OS
  8.25, HW 226. TR-064 services enumerated; DSL+WLAN+Hosts all present.
- Credentials: user provides via env `FRITZBOX_USERNAME` / `FRITZBOX_PASSWORD`.

## Open verification items (for implementation)
- Confirm `fritzbox.dsl.rate.current` unit (bit/s vs kb/s) against box UI.
- Confirm WLAN instance mapping (1=2.4GHz, 2=5GHz, 3=guest) live.
- Git: repo not yet initialized — `git init` before first commit.
