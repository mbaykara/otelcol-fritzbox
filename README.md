# otelcol-fritzbox

[![CI](https://github.com/mbaykara/otelcol-fritzbox/actions/workflows/ci.yml/badge.svg)](https://github.com/mbaykara/otelcol-fritzbox/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/mbaykara/otelcol-fritzbox.svg)](https://pkg.go.dev/github.com/mbaykara/otelcol-fritzbox)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

An [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) metrics
receiver that scrapes **AVM Fritz!Box** routers (and Fritz!Repeaters) via
their built-in **TR-064 API** — the native OTel equivalent of the existing
Prometheus exporters.

Metrics follow the
[`hw.network.*` semantic conventions](https://opentelemetry.io/docs/specs/semconv/hardware/network/)
where applicable; Fritz!Box-specific data (DSL physics, WLAN radios, hosts,
WAN connection) is exposed under `fritzbox.*`.

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
| `fritzbox.wan.external_ip` | Gauge | 1 | External IP in attribute (optional, off by default) |
| `fritzbox.dsl.rate.current` / `.rate.max` | Gauge | bit/s | DSL sync/max rate per direction |
| `fritzbox.dsl.noise_margin` / `.attenuation` | Gauge | dB | DSL SNR/attenuation per direction |
| `fritzbox.dsl.error_seconds` | Sum (monotonic) | s | Errored/severely-errored seconds |
| `fritzbox.wlan.channel` | Gauge | 1 | WLAN channel per radio (`hw.id`, `ssid`) |
| `fritzbox.wlan.clients` | Gauge | {client} | Associated clients per radio |
| `fritzbox.hosts.total` | Gauge | {host} | Known hosts |
| `fritzbox.hosts.active` | Gauge | {host} | Active hosts (optional; costs one SOAP call per host) |

Resource attributes: `hw.vendor=AVM`, `hw.model`, `hw.serial_number`,
`fritzbox.device.software_version`, `server.address`.

Cable/fiber boxes without a DSL service simply skip the DSL group; boxes
without WLAN skip WLAN metrics.

## Configuration

```yaml
receivers:
  fritzbox:
    endpoint: http://fritz.box:49000
    username: ${env:FRITZBOX_USERNAME}   # optional
    password: ${env:FRITZBOX_PASSWORD}   # optional
    collection_interval: 30s
    timeout: 10s
    metrics:
      fritzbox.hosts.active:
        enabled: true
```

**Auth:** Fritz!Box uses HTTP digest auth per service. Some actions (e.g.
host counts) work unauthenticated; device/DSL/WiFi metrics require
credentials. Without credentials, protected groups are skipped with a
one-time warning.

Create a dedicated user on the box under
*System → FRITZ!Box Users* with "FRIT!Box settings" permission.

## Using it in your collector

Add the receiver to your
[OCB](https://opentelemetry.io/docs/collector/custom-collector/) manifest:

```yaml
receivers:
  - gomod: github.com/mbaykara/otelcol-fritzbox/receiver/fritzbox v0.1.0
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

## Development

Requires Go 1.24+.

```sh
go generate ./...   # regenerate mdatagen code from metadata.yaml
go test ./...
go vet ./...
```

`example/` contains a ready-to-use OCB manifest and collector config for a
live smoke test against a real box:

```sh
cd example
go tool go.opentelemetry.io/collector/cmd/builder --config builder-config.yaml
FRITZBOX_USERNAME=... FRITZBOX_PASSWORD=... \
  ./otelcol-fritzbox/otelcol-fritzbox --config collector-config.yaml
```

## License

[Apache 2.0](LICENSE)
