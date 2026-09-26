#!/usr/bin/env bash
# Smoke-tests a built image: version output, validation of the bundled
# config, the non-root user, and a healthy health_check endpoint while the
# device is unreachable.
set -euo pipefail

image=${1:?usage: image-smoke.sh IMAGE}
env=(
  -e OTLP_ENDPOINT=https://otlp.invalid/otlp
  -e OTLP_USERNAME=smoke
  -e OTLP_PASSWORD=smoke
  -e FRITZBOX_ENDPOINT=http://192.0.2.1:49000
)

docker run --rm "$image" --version
docker run --rm "${env[@]}" "$image" validate --config=/etc/otelcol-fritzbox/config.yaml

user=$(docker image inspect "$image" --format '{{.Config.User}}')
if [[ "$user" != "65532:65532" ]]; then
  echo "image runs as '$user', want 65532:65532"
  exit 1
fi

cid=$(docker run -d -P "${env[@]}" "$image")
trap 'docker rm -f "$cid" >/dev/null' EXIT
port=$(docker port "$cid" 13133/tcp | head -1 | sed 's/.*://')
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$port/" >/dev/null; then
    echo "health_check healthy on port $port"
    exit 0
  fi
  sleep 1
done
docker logs "$cid"
echo "health_check did not become healthy"
exit 1
