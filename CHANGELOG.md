# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [0.2.0] - 2026-09-27

### Added

- Container image `ghcr.io/mbaykara/otelcol-fritzbox` (amd64, arm64, arm/v7), signed, with SBOM and provenance.
- Helm chart `oci://ghcr.io/mbaykara/charts/otelcol-fritzbox`, signed.
- `linux_armv7` release binaries.
- `tls` receiver settings for `https://` endpoints.
- `health_check` extension and `memory_limiter` processor in the distribution.
- `example/collector-config-production.yaml`, replacing `collector-config-e2e.yaml`.

### Changed

- `password` is redacted when the collector prints its configuration.
- Endpoints must use http or https; TLS settings on http are rejected.

### Fixed

- A failing metric group no longer drops the metrics of every other group.
- Missing credentials skip protected groups with one warning instead of failing each scrape.
- HTTP 401 and UPnP fault 401 are no longer conflated.
- `fritzbox.hosts.active` is not emitted from an incomplete host enumeration.
- An unreachable device no longer delays collector startup.

## [0.1.0] - 2026-09-07

First public release. Also goes by the informal codename "VDSL Works".

### Added

- `fritzbox` metrics receiver: scrapes AVM Fritz!Box routers via TR-064
  (SOAP + HTTP digest auth with nonce-count progression) and emits metrics
  following the `hw.network.*` semantic conventions plus `fritzbox.*` for
  DSL, WLAN, hosts, and device data.
- Metric groups: device uptime, WAN traffic/link status and speed, WAN
  connection status, DSL rates/noise margin/attenuation/error counters,
  WLAN channel/clients/packets per radio, host counts.
- Opt-in metrics (disabled by default): `fritzbox.hosts.info` (per-device
  identity), `fritzbox.hosts.active` (per-host enumeration), and
  `fritzbox.wan.external_ip` (public IP attribute).
- `otelcol-fritzbox` collector distribution (cmd/, GoReleaser binaries for
  linux/darwin on amd64/arm64).
- Example configs: minimal (privacy-preserving), full debug, and Grafana
  Cloud e2e (OTLP). Grafana dashboard with device table under `dashboard/`.
- CI (gofmt, vet, tests, mdatagen freshness, distribution build) and
  tag-triggered release workflows.
- Documentation: validated quickstart with expected first output, privacy
  section, troubleshooting guide, tested-device matrix, CONTRIBUTING.md,
  SECURITY.md, AGENTS.md engineering policy.

### Fixed

- Deadlock on unauthenticated devices: the resource cache held a mutex
  across a call path that logs via the same mutex (regression test added).
- SOAP faults wrapped in HTTP 500 are now parsed as typed errors; faulting
  advertised actions (e.g. WANIPConnection on some boxes) skip their group
  with a one-time warning instead of failing the scrape.
- Digest authentication reuses the cached challenge preemptively and
  increments the nonce count, as Fritz!Box enforces replay protection.
- DSL rates are converted from TR-064 kbit/s to UCUM bit/s.
- The distribution's `validate` command works (telemetry factory was missing
  from the hand-written component wiring).
