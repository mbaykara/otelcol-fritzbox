#!/usr/bin/env bash
# Renders the Helm chart for a set of value combinations, validates every
# rendered collector config with the distribution binary, checks manifests
# with kubeconform when available, and asserts that invalid values fail.
set -euo pipefail

cd "$(dirname "$0")/.."
chart=charts/otelcol-fritzbox
bin=${OTELCOL_BIN:-bin/otelcol-fritzbox}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

export MY_POD_IP=127.0.0.1 FRITZBOX_USERNAME=u FRITZBOX_PASSWORD=p OTLP_USERNAME=u OTLP_PASSWORD=p

render_ok() {
  local name=$1; shift
  helm template t "$chart" "$@" > "$tmp/$name.yaml"
  # Extract the collector config from the ConfigMap.
  yq 'select(.kind == "ConfigMap") | .data["config.yaml"]' "$tmp/$name.yaml" > "$tmp/$name.config.yaml"
  "$bin" validate --config="file:$tmp/$name.config.yaml"
  if command -v kubeconform >/dev/null; then
    kubeconform -strict -summary -skip ServiceMonitor "$tmp/$name.yaml"
  fi
  echo "ok   $name"
}

render_fails() {
  local name=$1 want=$2; shift 2
  local out
  if out=$(helm template t "$chart" "$@" 2>&1); then
    echo "FAIL $name: rendering succeeded, want error containing '$want'"
    exit 1
  fi
  if [[ "$out" != *"$want"* ]]; then
    echo "FAIL $name: error does not contain '$want':"
    echo "$out"
    exit 1
  fi
  echo "ok   $name (rejected)"
}

for values in "$chart"/ci/*-values.yaml; do
  render_ok "$(basename "$values" -values.yaml)" -f "$values"
done
render_ok existing-secrets \
  --set fritzbox.existingSecret=fritzbox --set otlp.existingSecret=otlp \
  --set otlp.endpoint=https://otlp.example.com/otlp
render_ok tls-configmap \
  --set fritzbox.endpoint=https://192.168.178.1:49443 \
  --set fritzbox.tls.caConfigMap=fritzbox-ca --set fritzbox.tls.serverNameOverride=fritz.box
render_ok otlp-no-auth --set otlp.endpoint=http://collector:4318 --set otlp.auth=none
render_ok digest --set image.digest=sha256:0000000000000000000000000000000000000000000000000000000000000000

# The chart must not create a Secret when every credential comes from an existing one.
if yq -e 'select(.kind == "Secret")' "$tmp/existing-secrets.yaml" >/dev/null 2>&1; then
  echo "FAIL existing-secrets: chart created a Secret"
  exit 1
fi
if ! grep -q 'ca_file: /etc/otelcol-fritzbox/tls/ca.crt' "$tmp/tls-configmap.config.yaml"; then
  echo "FAIL tls-configmap: ca_file missing from config"
  exit 1
fi

render_fails half-credentials "must be set together" --set fritzbox.username=u
render_fails basic-auth-without-credentials "otlp.auth=basic needs" --set otlp.endpoint=https://otlp.example.com
render_fails two-ca-sources "only one of" --set fritzbox.tls.caSecret=a --set fritzbox.tls.caConfigMap=b
render_fails bad-endpoint "endpoint" --set fritzbox.endpoint=fritz.box
render_fails bad-auth "auth" --set otlp.auth=bearer
