# otelcol-fritzbox Helm chart

Runs the [otelcol-fritzbox](https://github.com/mbaykara/otelcol-fritzbox)
collector against one AVM Fritz!Box and ships the metrics to an OTLP/HTTP
backend such as Grafana Cloud.

The chart always runs exactly one replica with the `Recreate` strategy. A
second pod would scrape and export the same router twice, so there is no
`replicaCount`. Install one release per router.

## Install

The cluster must be able to reach the router. Use its IP address if
`fritz.box` does not resolve inside the cluster.

```sh
kubectl create secret generic fritzbox \
  --from-literal=username=<fritzbox-user> --from-literal=password=<fritzbox-password>
kubectl create secret generic grafana-cloud \
  --from-literal=username=<instance-id> --from-literal=password=<access-policy-token>

helm install fritzbox oci://ghcr.io/mbaykara/charts/otelcol-fritzbox --version <version> \
  --set fritzbox.endpoint=http://192.168.178.1:49000 \
  --set fritzbox.existingSecret=fritzbox \
  --set otlp.endpoint=https://otlp-gateway-prod-eu-west-2.grafana.net/otlp \
  --set otlp.existingSecret=grafana-cloud
```

Without `otlp.endpoint` the chart uses the debug exporter, so metrics appear
in the pod log. Without Fritz!Box credentials only unauthenticated groups
(host counts) are collected.

The chart is signed; see
[signature verification](https://github.com/mbaykara/otelcol-fritzbox#container-image).

## Values

| Key | Default | Description |
|---|---|---|
| `fritzbox.endpoint` | `http://fritz.box:49000` | TR-064 URL, `http://<box>:49000` or `https://<box>:49443` |
| `fritzbox.existingSecret` | `""` | Secret with the Fritz!Box credentials, keys set by `fritzbox.existingSecretKeys` (`username`, `password`) |
| `fritzbox.username` / `fritzbox.password` | `""` | Inline credentials, the chart creates a Secret. Prefer `existingSecret` |
| `fritzbox.collectionInterval` / `fritzbox.timeout` | `30s` / `10s` | Scrape interval and per-scrape deadline |
| `fritzbox.tls.caSecret` / `fritzbox.tls.caConfigMap` | `""` | Secret or ConfigMap with the certificate exported from the box, key `fritzbox.tls.caKey` (`ca.crt`) |
| `fritzbox.tls.serverNameOverride` | `""` | Certificate name to verify when connecting by IP, usually `fritz.box` |
| `fritzbox.metrics` | `{}` | Metric toggles, e.g. `{"fritzbox.hosts.info": {"enabled": true}}` |
| `otlp.endpoint` | `""` | OTLP/HTTP endpoint. Empty means debug exporter |
| `otlp.auth` | `basic` | `basic` or `none` |
| `otlp.existingSecret` | `""` | Secret with the OTLP basic auth credentials, keys set by `otlp.existingSecretKeys` |
| `otlp.username` / `otlp.password` | `""` | Inline OTLP credentials, the chart creates a Secret |
| `memoryLimiter.limitPercentage` / `spikeLimitPercentage` | `80` / `25` | `memory_limiter` limits as a share of the container memory limit |
| `config` | `{}` | Collector config deep-merged over the generated one. Lists replace lists |
| `image.repository` / `tag` / `digest` | `ghcr.io/mbaykara/otelcol-fritzbox` / appVersion / `""` | `digest` takes precedence over `tag` |
| `resources` | requests 10m CPU, 64Mi; limit 256Mi | Container resources |
| `serviceMonitor.enabled` | `false` | ServiceMonitor for the collector's own metrics on `:8888` |
| `podSecurityContext` / `securityContext` | non-root 65532, read-only root, no capabilities, RuntimeDefault seccomp | Compatible with the `restricted` Pod Security Standard |


## Adding exporters or processors

`config` is merged into the generated collector configuration. To send to a
second backend:

```yaml
config:
  exporters:
    otlp_http/second:
      endpoint: https://otlp.example.com
  service:
    pipelines:
      metrics:
        exporters: [otlp_http, otlp_http/second]
```

## Ports

| Port | Name | Purpose |
|---|---|---|
| 13133 | `health` | `health_check` extension, used by the liveness and readiness probes |
| 8888 | `metrics` | Collector self-telemetry in Prometheus format |
