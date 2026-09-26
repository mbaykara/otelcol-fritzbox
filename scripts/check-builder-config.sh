#!/usr/bin/env bash
# Fails when example/builder-config.yaml drifts from go.mod or from the
# component factories registered in cmd/otelcol-fritzbox/components.go.
set -euo pipefail

cd "$(dirname "$0")/.."
config=example/builder-config.yaml
components=cmd/otelcol-fritzbox/components.go
self=$(go list -m)
status=0

want=$(awk '/^  otelcol_version:/ {print $2}' "$config")
have=$(go list -m -f '{{.Version}}' go.opentelemetry.io/collector/otelcol)
if [[ "$want" != "$have" ]]; then
  echo "$config: otelcol_version $want, go.mod has $have"
  status=1
fi

while read -r module version; do
  [[ "$module" == "$self" ]] && continue
  have=$(go list -m -f '{{.Version}}' "$module")
  if [[ "$version" != "$have" ]]; then
    echo "$config: $module $version, go.mod has $have"
    status=1
  fi
  if ! grep -q "\"$module\"" "$components"; then
    echo "$components: $module is in $config but not registered"
    status=1
  fi
done < <(awk '/- gomod:/ {print $3, $4}' "$config")

exit "$status"
