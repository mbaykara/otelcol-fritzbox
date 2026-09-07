# continue-here

## Project
fritzotel-receiver — OTel Collector metrics receiver for AVM Fritz!Box
routers (TR-064). Spec: `docs/superpowers/specs/2026-09-07-fritzbox-receiver-design.md`.

## Current state (2026-09-07 ~09:35)
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

## Known limitations
- WAN connection status/uptime unavailable on this box (TR-064 service
  advertised but faults).
- WAN utilization skipped (auth scope).
- `fritzbox.wan.external_ip` disabled by default.
