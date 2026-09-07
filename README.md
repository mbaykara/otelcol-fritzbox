# otelcol-fritzbox

[![CI](https://github.com/mbaykara/otelcol-fritzbox/actions/workflows/ci.yml/badge.svg)](https://github.com/mbaykara/otelcol-fritzbox/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/mbaykara/otelcol-fritzbox.svg)](https://pkg.go.dev/github.com/mbaykara/otelcol-fritzbox)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

An [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) metrics
receiver for **AVM Fritz!Box** routers (and Fritz!Repeaters) via their built-in
**TR-064 API** — plus `otelcol-fritzbox`, a ready-to-run collector distribution
that bundles it.

Metrics follow the
[`hw.network.*` semantic conventions](https://opentelemetry.io/docs/specs/semconv/hardware/network/)
where applicable; Fritz!Box-specific data (DSL physics, WLAN radios, hosts)
is exposed under `fritzbox.*`.

## Quickstart (validated)

**1. Download and verify the collector** (or [build from source](#development)):

```sh
# pick your platform: darwin_arm64, darwin_amd64, linux_amd64, linux_arm64
VERSION=0.1.1
PLATFORM=darwin_arm64
curl -sLO "https://github.com/mbaykara/otelcol-fritzbox/releases/download/v${VERSION}/otelcol-fritzbox_${VERSION}_${PLATFORM}.tar.gz"
curl -sLO "https://github.com/mbaykara/otelcol-fritzbox/releases/download/v${VERSION}/otelcol-fritzbox_${VERSION}_checksums.txt"
# verify: the checksum printed must match the tarball's line in checksums.txt
shasum -a 256 "otelcol-fritzbox_${VERSION}_${PLATFORM}.tar.gz"
grep "${PLATFORM}" otelcol-fritzbox_${VERSION}_checksums.txt
tar xzf "otelcol-fritzbox_${VERSION}_${PLATFORM}.tar.gz"
```

**2. Configure.** Create a dedicated user on the box under
*System → FRITZ!Box Users* with "FRITZ!Box settings" permission, then:

```sh
export FRITZBOX_USERNAME=myuser FRITZBOX_PASSWORD=mypassword
cp example/collector-config-minimal.yaml config.yaml
```

The minimal config collects aggregated, low-cardinality metrics only: no
per-device identity, no external IP. See [Privacy](#privacy).

**3. Validate and run:**

```sh
./otelcol-fritzbox validate --config config.yaml   # config check, no scraping
./otelcol-fritzbox --config config.yaml            # runs and prints metrics
```

**Expected first output** (within ~30 s, debug exporter):

```text
info    fritzbox/scraper.go:101    fritzbox: discovered TR-064 services ... count": 37
...
Metrics ... "metrics": 14, "data points": 39
```

If you see this, the pipeline works end to end. For anything else, see
[Troubleshooting](#troubleshooting).

## Collected metrics

| Metric | Type | Unit | Description |
|---|---|---|---|
| `hw.network.io` | Sum (monotonic) | By | WAN bytes received/transmitted (`network.io.direction`) |
| `hw.network.packets` | Sum (monotonic) | {packet} | WAN + WLAN packets (`hw.id`, `network.io.direction`) |
| `hw.network.bandwidth.limit` | Gauge | By/s | WAN link speed |
| `hw.network.bandwidth.utilization` | Gauge | 1 | WAN bandwidth utilization fraction |
| `hw.network.up` | Gauge | 1 | Link status (WAN + per WLAN radio) |
| `hw.errors` | Sum (monotonic) | {error} | DSL line errors (`error.type=fec/crc/hec`, direction) |
| `fritzbox.device.uptime` | Gauge | s | Device uptime |
| `fritzbox.wan.connection.status` | Gauge | 1 | 1 = Connected |
| `fritzbox.wan.connection.uptime` | Gauge | s | WAN connection uptime |
| `fritzbox.wan.external_ip` | Gauge | 1 | **Opt-in.** External IP in attribute |
| `fritzbox.dsl.rate.current` / `.rate.max` | Gauge | bit/s | DSL sync/max rate per direction |
| `fritzbox.dsl.noise_margin` / `.attenuation` | Gauge | dB | DSL SNR/attenuation per direction |
| `fritzbox.dsl.error_seconds` | Sum (monotonic) | s | Errored/severely-errored seconds |
| `fritzbox.wlan.channel` | Gauge | 1 | WLAN channel per radio (`hw.id`, `ssid`) |
| `fritzbox.wlan.clients` | Gauge | {client} | Associated clients per radio |
| `fritzbox.hosts.total` | Gauge | {host} | Known hosts |
| `fritzbox.hosts.active` | Gauge | {host} | **Opt-in.** Active hosts (N extra SOAP calls) |
| `fritzbox.hosts.info` | Gauge | 1 | **Opt-in.** Per-device identity: hostname, ip, mac, interface_type, active, guest, friendly_name |

Resource attributes: `hw.vendor=AVM`, `hw.model`, `hw.serial_number`,
`fritzbox.device.software_version`, `server.address`.

Cable/fiber boxes without a DSL service simply skip the DSL group; boxes
without WLAN skip WLAN metrics.

## Privacy

Two metrics are **opt-in** (disabled by default) because they export
identifying information:

- `fritzbox.hosts.info` — one series per known device carrying its hostname,
  IP, MAC address, guest status, and friendly name. Sends your household's
  device inventory to the configured backend and creates one series per
  device (label churn when DHCP addresses change).
- `fritzbox.wan.external_ip` — publishes your public IP as a metric attribute.

Enable them explicitly only where you want this data:

```yaml
receivers:
  fritzbox:
    metrics:
      fritzbox.hosts.info:
        enabled: true
```

`example/collector-config.yaml` enables all opt-in metrics for demonstration;
`example/collector-config-minimal.yaml` enables none.

## Grafana dashboard

A ready-made dashboard (`dashboard/fritzbox-network.json`, incl. a device
table) is in this repo. Import via *Dashboards → New → Import* and select your
Prometheus datasource, or manage it as code with
[gcx](https://github.com/grafana/gcx):

```sh
gcx dashboards create --context <your-context> -f dashboard/fritzbox-network-k8s.json
```

The dashboard queries Prometheus-style names — Grafana Cloud converts OTLP
metric names on ingestion (dots become underscores, units become suffixes):
`fritzbox.device.uptime` → `fritzbox_device_uptime_seconds`.

## Using the receiver in your own collector

Add it to your [OCB](https://opentelemetry.io/docs/collector/custom-collector/)
manifest — `gomod` is the Go module, `import` is the receiver package:

```yaml
receivers:
  - gomod: github.com/mbaykara/otelcol-fritzbox v0.1.1
    import: github.com/mbaykara/otelcol-fritzbox/receiver/fritzbox
```

Then wire it into a metrics pipeline:

```yaml
service:
  pipelines:
    metrics:
      receivers: [fritzbox]
      processors: [batch]
      exporters: [otlp]
```

## Authentication

Fritz!Box uses HTTP digest auth **per service**. Some actions (host counts)
work unauthenticated; device/DSL/WiFi metrics require credentials. Without
credentials, protected groups are skipped with a one-time warning.

- `401` in logs on first scrape of a group → username/password missing or wrong.
- Digest nonces rotate; the client handles nonce-count progression automatically.

## Troubleshooting

**`context deadline exceeded` on discovery** — the box is unreachable (wrong
endpoint, wrong network) or DNS for `fritz.box` resolves to a public parked
address (some DNS setups hijack it). Use the box's IP directly.

**A metric group is missing entirely** — the device doesn't offer that
service (e.g. no `WANDSLInterfaceConfig` on cable/fiber boxes) or the action
faults. The log contains a one-time `warn` per skipped group naming the
service. This is a device capability difference, not an exporter failure.

**`UPnP error 401: Invalid Action` for WAN connection metrics** — some
firmware/WAN-mode combinations advertise `WANIPConnection` but don't
implement its actions. The group is skipped; everything else keeps working.

**Router down vs exporter down** — if the router is unreachable, the
collector logs `Error scraping metrics ... fetching device description` once
per interval and emits nothing. The collector process itself stays up and
reports its own health via its self-telemetry on `:8888`. If the collector
process is down, no logs and no self-telemetry.

## Tested devices

| Model | FRITZ!OS | Connection | Verified groups | Notes |
|---|---|---|---|---|
| FRITZ!Box 7590 (HW 226) | 8.25 | VDSL | device, wan traffic/link, dsl, wlan (3 radios), hosts, host info | WAN connection status/uptime unavailable (service faults); utilization requires extra permission scope |

Expected to work but **not verified**: other 75xx/56xx/66xx models on
FRITZ!OS 7.5+, cable and fiber variants (DSL group auto-skips), and
Fritz!Repeater devices (subset: device, wlan, hosts). Reports welcome.

## Development

Requires Go 1.27.

```sh
go generate ./receiver/...   # regenerate mdatagen code from metadata.yaml
go test ./...
go vet ./...
```

`example/` contains an OCB manifest and collector configs:

```sh
cd example
go tool go.opentelemetry.io/collector/cmd/builder --config builder-config.yaml
./otelcol-fritzbox/otelcol-fritzbox --config collector-config.yaml
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Report security issues privately per
[SECURITY.md](SECURITY.md).

## License

[Apache 2.0](LICENSE)
