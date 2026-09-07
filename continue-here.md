# continue-here

## Project
fritzotel-receiver — custom OTel Collector **metrics receiver** for AVM
Fritz!Box routers (TR-064 API). Spec: `docs/superpowers/specs/2026-09-07-fritzbox-receiver-design.md`.

## Current state (2026-09-07 ~02:05)
**Receiver fully implemented, unit-tested, OCB-buildable. Awaiting live
credential test.**

- `receiver/fritzbox/` — config.go, factory.go, scraper.go, doc.go,
  generated mdatagen code (all tests green, vet/gofmt clean).
- `receiver/fritzbox/internal/tr064/` — SOAP client + RFC 7616 MD5 digest
  auth; full test coverage incl. digest retry, faults, input args.
- `example/` — OCB manifest (`builder-config.yaml`) + collector config;
  `example/otelcol-fritzbox/` binary builds successfully.
- LICENSE (Apache 2.0), README, CI stub committed.
- 5 commits on `main`. Git repo local-only (no remote yet).

## What works
- `go test ./...` all green (scraper + tr064 + generated component tests).
- OCB build: `cd example && go tool go.opentelemetry.io/collector/cmd/builder
  --config builder-config.yaml`.
- Collector starts, receiver runs, discovery/auth errors surface via
  collector telemetry (verified unauthenticated).

## Next step (blocked on user)
**Live smoke test with credentials:** run the example collector with
`FRITZBOX_USERNAME` / `FRITZBOX_PASSWORD` env vars set, confirm
device/DSL/WLAN groups emit, and verify:
1. `fritzbox.dsl.rate.current` unit (bit/s vs kb/s) against box UI
2. WLAN instance mapping (1=2.4GHz, 2=5GHz, 3=guest)

Then tag `v0.1.0`.

## Key decisions (from spec)
- Mixed metric scheme: `hw.network.*` semconv + `fritzbox.*` custom.
- Module path: `github.com/mbaykara/fritzotel-receiver`, receiver at
  `receiver/fritzbox` (single module; OCB uses gomod=repo + import=path).
- Credentials optional (per-service auth); 401 groups skipped w/ warn-once.
- `mdatagen`+`builder` as Go tool deps.
