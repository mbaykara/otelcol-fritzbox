# Installation

## Binary (validated quickstart)

**1. Download and verify the collector** (or build from source, see [CONTRIBUTING.md](../CONTRIBUTING.md)):

```sh
# pick your platform: darwin_arm64, darwin_amd64, linux_amd64, linux_arm64, linux_armv7
VERSION=0.2.0
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
per-device identity, no external IP. See [Privacy](configuration.md#privacy).

**3. Validate and run:**

```sh
./otelcol-fritzbox validate --config config.yaml   # config check, no scraping
./otelcol-fritzbox --config config.yaml            # runs and prints metrics
```

**Expected first output** (within ~30 s, debug exporter):

```text
info    fritzbox: discovered TR-064 services ... count": 37
...
Metrics ... "metrics": 14, "data points": 39
```

If you see this, the pipeline works end to end. For anything else, see
[Troubleshooting](troubleshooting.md).

## Container image

Multi-arch images (`linux/amd64`, `linux/arm64`, `linux/arm/v7`) are
published to `ghcr.io/mbaykara/otelcol-fritzbox` for every release after
0.1.0. They are distroless, run as UID 65532, and bundle
`example/collector-config-production.yaml` as the default configuration, so
a deployment only needs environment variables:

```sh
docker run -d --name otelcol-fritzbox --restart unless-stopped \
  -e FRITZBOX_ENDPOINT=http://192.168.178.1:49000 \
  -e FRITZBOX_USERNAME -e FRITZBOX_PASSWORD \
  -e OTLP_ENDPOINT=https://otlp-gateway-prod-eu-west-2.grafana.net/otlp \
  -e OTLP_USERNAME -e OTLP_PASSWORD \
  -p 13133:13133 \
  ghcr.io/mbaykara/otelcol-fritzbox:<version>
```

Mount your own file over `/etc/otelcol-fritzbox/config.yaml` to replace the
configuration. `:13133` is the `health_check` endpoint.

Images and charts carry cosign keyless signatures; images also carry an SBOM
and SLSA provenance attestation. Verify with (for the chart, use
`ghcr.io/mbaykara/charts/otelcol-fritzbox:<version>`):

```sh
cosign verify ghcr.io/mbaykara/otelcol-fritzbox:<version> \
  --certificate-identity-regexp '^https://github.com/mbaykara/otelcol-fritzbox/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Kubernetes

The Helm chart is published to `oci://ghcr.io/mbaykara/charts/otelcol-fritzbox`.
Install, secrets, TLS, values, and a Flux example:
[charts/otelcol-fritzbox/README.md](../charts/otelcol-fritzbox/README.md).
