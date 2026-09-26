# syntax=docker/dockerfile:1

# The build stage runs on the build host and cross-compiles, so multi-arch
# builds need no emulation.
FROM --platform=$BUILDPLATFORM golang:1.27.1-bookworm@sha256:69a7b9788769bec032d238959b61854e9ae87f57be9029ec04e9885fabf99195 AS build

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd/ cmd/
COPY internal/ internal/
COPY receiver/ receiver/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOARM="${TARGETVARIANT#v}" \
    CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/otelcol-fritzbox ./cmd/otelcol-fritzbox

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

LABEL org.opencontainers.image.title="otelcol-fritzbox" \
      org.opencontainers.image.description="OpenTelemetry Collector with the Fritz!Box receiver" \
      org.opencontainers.image.source="https://github.com/mbaykara/otelcol-fritzbox" \
      org.opencontainers.image.licenses="Apache-2.0"

COPY --from=build /out/otelcol-fritzbox /otelcol-fritzbox
COPY example/collector-config-production.yaml /etc/otelcol-fritzbox/config.yaml

# health_check must listen beyond loopback for container probes.
ENV HEALTH_CHECK_ENDPOINT=0.0.0.0:13133

USER 65532:65532
EXPOSE 13133

ENTRYPOINT ["/otelcol-fritzbox"]
CMD ["--config=/etc/otelcol-fritzbox/config.yaml"]
