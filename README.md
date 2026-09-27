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

![Fritz!Box home network dashboard](dashboard/fritzbox-modern.png)

## Install

Create a user on the box under *System → FRITZ!Box Users* with the
"FRITZ!Box settings" permission, then run the collector one of these ways.

**Kubernetes (Helm):**

```sh
kubectl create secret generic fritzbox --from-literal=username=<user> --from-literal=password=<password>
kubectl create secret generic otlp --from-literal=username=<instance-id> --from-literal=password=<token>
helm install fritzbox oci://ghcr.io/mbaykara/charts/otelcol-fritzbox \
  --set fritzbox.endpoint=http://192.168.178.1:49000 --set fritzbox.existingSecret=fritzbox \
  --set otlp.endpoint=https://otlp-gateway-prod-eu-west-2.grafana.net/otlp --set otlp.existingSecret=otlp
```

**Docker:**

```sh
docker run -d --restart unless-stopped -p 13133:13133 \
  -e FRITZBOX_ENDPOINT=http://192.168.178.1:49000 -e FRITZBOX_USERNAME -e FRITZBOX_PASSWORD \
  -e OTLP_ENDPOINT=https://otlp-gateway-prod-eu-west-2.grafana.net/otlp -e OTLP_USERNAME -e OTLP_PASSWORD \
  ghcr.io/mbaykara/otelcol-fritzbox:0.2.0
```

**Binary:** download from [Releases](https://github.com/mbaykara/otelcol-fritzbox/releases)
and run it with a config from [`example/`](example/), see the
[validated quickstart](docs/installation.md#binary-validated-quickstart).

## Dashboard

Import [`dashboard/fritzbox-modern.json`](dashboard/fritzbox-modern.json) via
*Dashboards → New → Import*. It needs the
[Business Charts](https://grafana.com/grafana/plugins/volkovlabs-echarts-panel/)
plugin and the opt-in `fritzbox.hosts.info` and `fritzbox.hosts.active`
metrics for the device panels. Details: [docs/dashboards.md](docs/dashboards.md).

## Documentation

- [Installation](docs/installation.md): binary, container image, signatures, Kubernetes
- [Configuration](docs/configuration.md): receiver options, TLS, authentication, privacy
- [Metrics](docs/metrics.md): metric reference and tested devices
- [Dashboards](docs/dashboards.md)
- [Using the receiver in your own collector](docs/custom-collector.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Helm chart](charts/otelcol-fritzbox/README.md)

## Contributing

Checks, tests, and conventions: [CONTRIBUTING.md](CONTRIBUTING.md). Report
security issues privately per [SECURITY.md](SECURITY.md).

## License

[Apache 2.0](LICENSE)
