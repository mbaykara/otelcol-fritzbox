# Configuration

| Key | Default | Description |
|---|---|---|
| `endpoint` | `http://fritz.box:49000` | TR-064 base URL. `http://<box>:49000` or `https://<box>:49443` |
| `username` / `password` | empty | Digest credentials. Both or neither. The password is redacted when the config is printed |
| `tls` | system roots | Certificate settings for an `https` endpoint (`ca_file`, `ca_pem`, `server_name_override`, `insecure_skip_verify`, ...). Rejected on `http` |
| `collection_interval` | `30s` | Scrape interval |
| `timeout` | `10s` | Per-scrape deadline, must not exceed `collection_interval` |
| `initial_delay` | `1s` | Delay before the first scrape |
| `metrics.<name>.enabled` | see [metrics](metrics.md) | Toggle individual metrics |

**TLS.** The box serves TR-064 over TLS on port 49443 with a self-signed
certificate. Export it from the box UI (*Internet → Permit Access → FRITZ!Box
Services*, certificate download) and point `ca_file` at it. If the certificate
names `fritz.box` but you connect by IP, set `server_name_override: fritz.box`.
`insecure_skip_verify: true` disables verification and sends digest responses
to whoever answers; avoid it outside of a lab.

```yaml
receivers:
  fritzbox:
    endpoint: https://192.168.178.1:49443
    username: ${env:FRITZBOX_USERNAME}
    password: ${env:FRITZBOX_PASSWORD}
    tls:
      ca_file: /etc/otelcol-fritzbox/fritzbox-ca.pem
      server_name_override: fritz.box
```

[`example/collector-config-production.yaml`](../example/collector-config-production.yaml) is a complete long-running setup:
`memory_limiter`, `batch`, `health_check` on `:13133`, and an OTLP/HTTP
exporter with basic auth, all driven by environment variables.

## Authentication

Fritz!Box uses HTTP digest auth **per service**. Some actions (host counts)
work unauthenticated; device/DSL/WiFi metrics require credentials.

- No credentials configured: protected groups are skipped with a one-time
  `requires credentials` warning. Everything else is exported.
- Credentials configured but rejected: every scrape logs
  `device rejected credentials` as an error. Unaffected groups are still
  exported.
- Digest nonces rotate; the client handles nonce-count progression automatically.

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
