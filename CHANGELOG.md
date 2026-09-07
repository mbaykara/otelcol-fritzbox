# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

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
