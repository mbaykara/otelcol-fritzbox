# continue-here

## Project
otelcol-fritzbox — OTel Collector metrics receiver for AVM Fritz!Box
routers (TR-064). Spec: `specs/2026-09-07-fritzbox-receiver-design.md`.

## Current state (2026-09-07 ~13:30)
**E2E validated: receiver -> collector -> Grafana Cloud -> dashboard.**

- Receiver: complete, unit-tested, tagged `v0.1.0`.
- Live verified against Fritz!Box 7590 (Fritz!OS 8.25, VDSL).
- Metrics flowing to Grafana Cloud stack `baykara` (prod-eu-west-2).
- Dashboard `fritzbox-network` created and validated: **15/15 panel queries
  return data**.
- Grafana Cloud naming: dots->underscores + unit suffixes
  (`fritzbox_dsl_rate_current_bit_per_second`, `hw_network_io_bytes_total`,
  `hw_network_up_ratio`, ...).
- gcx context `baykara` created (OAuth), prometheus remote_write used
  (OTLP gateway returned 401 with that token).

## Repo layout
- `receiver/fritzbox/` — receiver code + generated mdatagen + tests.
- `receiver/fritzbox/internal/tr064/` — SOAP/digest client + tests.
- `example/` — OCB builder-config + collector configs (dev + e2e).
- `dashboard/fritzbox-network.json` — raw Grafana JSON.
- `dashboard/fritzbox-network-k8s.json` — K8s manifest (what gcx created).

## To re-run e2e
```sh
cd example
go tool go.opentelemetry.io/collector/cmd/builder --config builder-config.yaml
# needs: FRITZBOX_PASSWORD and GCOM_TOKEN files in $TMPDIR/opencode/
FRITZBOX_USERNAME=mbaykara FRITZBOX_PASSWORD=... GCOM_TOKEN=glc_... \
  ./otelcol-fritzbox/otelcol-fritzbox --config collector-config-e2e.yaml
```

## Device table (added 2026-09-07 ~11:12)
- Metric: fritzbox.hosts.info (gauge=1 per host, attrs hostname/ip/mac/interface_type/active/guest/friendly_name).
- Source: X_AVM-DE_GetHostListPath -> /devicehostlist.lua?sid=... (single call, ~73 hosts).
- Dashboard panel 15 "Connected Devices" (table, sorted Active desc) validated via gcx snapshot.

## Review fixes + release (2026-09-07 ~11:45)
- External review findings fixed (commit 8486262):
  - HIGH: resourceOptions deadlock (s.mu held across callGroup->warnOnce). Fixed + regression test.
  - HIGH: fritzbox.hosts.info now opt-in (privacy/cardinality); example configs enable it explicitly.
  - MED: digest auth preemptive once challenge cached (no per-call 401 round trip).
  - MED: recordUtilization propagates scrape ctx.
- v0.1.0 released: CI + Release green, binaries for linux/darwin amd64/arm64 on GitHub Releases.
- Repo: github.com/mbaykara/otelcol-fritzbox (private). Module path renamed from fritzotel-receiver.
- v0.1.1 released (docs/layout/AGENTS.md policy). CI + Release workflows green on both tags.
- Spec lives at specs/ (docs/superpowers removed).
- Dashboard link on baykara stack updated to new repo URL.

## Known limitations
- WAN connection status/uptime unavailable on this box (TR-064 service
  advertised but faults).
- WAN utilization skipped (auth scope).
- `fritzbox.wan.external_ip` disabled by default.

## Releases reset (2026-09-07 ~13:30)
All tags and releases (v0.1.0, v0.1.1, v0.1.2) were deleted; the v0.1.2
release workflow was cancelled. CHANGELOG.md (Unreleased section) documents
everything since project start. A proper release will be cut later today —
when tagging, rename the Unreleased section in CHANGELOG.md first.
