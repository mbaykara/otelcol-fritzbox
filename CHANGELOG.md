# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed

- A failing metric group no longer drops the whole scrape. Real failures are
  reported as partial scrape errors, so the collector still exports metrics
  from unaffected groups. Before, any group error (for example running
  without credentials) discarded every metric of that interval.
- Actions that need credentials are skipped with a one-time warning when no
  credentials are configured, instead of failing each scrape. Rejected
  credentials are reported as a scrape error on every interval.
- HTTP 401 and UPnP fault 401 (Invalid Action) are no longer conflated.
- `fritzbox.hosts.active` is not emitted when host enumeration fails part way,
  instead of reporting an undercount.

### Added

- `make check` runs the full CI suite locally (lint, race tests,
  govulncheck, generated-code and builder-config drift, build).
- In-memory fake Fritz!Box (`internal/fakebox`) with receiver integration
  tests through the scraper controller and a distribution end-to-end test.
- Issue forms, automatic issue and pull request labeling, Dependabot, pull
  request template, and CODEOWNERS.

### Changed

- CI actions are pinned to commit SHAs and run with read-only permissions.

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
