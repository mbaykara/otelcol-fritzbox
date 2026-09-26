# Contributing

## Checks

Requires the Go version in `go.mod`. `make help` lists all targets.

| Target | Runs | When |
|---|---|---|
| `make check` | generated-code freshness, builder-config drift, golangci-lint, race tests, govulncheck, build | every Go change |
| `make image-smoke` | builds the image, runs `scripts/image-smoke.sh` | `Dockerfile` or bundled config changes |
| `make chart-lint` | `helm lint`, `ct lint`, `scripts/chart-render-test.sh` | chart changes |
| `make chart-test` | installs the chart into kind (own kubeconfig in `bin/`) and runs `helm test` | template changes |

Lint and vulnerability tools run through `go run` at pinned versions. The
image and chart targets need Docker, helm, ct, kind, and yq.

## Tests without a router

`internal/fakebox` is an in-memory TR-064 device (digest auth, nonce rotation,
UPnP faults, host list). The unit tests, the controller-level tests in
`receiver/fritzbox/integration_test.go`, and the distribution test in
`cmd/otelcol-fritzbox/main_test.go` all use it. Model device behavior by
changing `fakebox.Box` fields in a test.

## Metric schema changes

Edit `receiver/fritzbox/metadata.yaml`, run `make generate`, and commit the
generated result in the same change.

## Collector dependencies

`go.mod`, `example/builder-config.yaml`, and
`cmd/otelcol-fritzbox/components.go` must agree; `make builder-check`
enforces it. Dependency bots do not edit the builder config, so update it in
the same pull request.

## Helm chart

Add a file to `charts/otelcol-fritzbox/ci/` for each value combination CI
should install. Leave the chart version alone; the release sets it from the
tag.

## Commits and PRs

- [Conventional Commits](https://www.conventionalcommits.org/), short
  single-sentence messages.
- One concern per PR. New behavior needs a focused test; bug fixes a
  regression test that fails on the old behavior where practical.
- No credentials, captured device data, or build artifacts.

Issue forms add a type label and `needs-triage`; the Label workflow adds
`area/*` labels to issues and PRs. Labels are only added, never removed.

## Device coverage

Tested a device? Open a device report so it can go into the README support
matrix.

## Releases

Maintainers cut releases with `v*` tags, which build binaries, the image,
and the chart. Do not push tags in PRs.
