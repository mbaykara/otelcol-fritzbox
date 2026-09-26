# Contributing

## Getting started

Requires the Go version declared in `go.mod`. Run everything CI runs:

```sh
make check
```

`make help` lists the individual targets (`lint`, `test`, `test-race`, `vuln`,
`build`, ...). Lint and vulnerability tools run through `go run` at pinned
versions, so no separate installation is needed.

## Tests without a router

`internal/fakebox` is an in-memory TR-064 device with digest authentication,
nonce rotation, UPnP faults, and the host list. It backs two test layers:

- `receiver/fritzbox/integration_test.go` runs the receiver through the real
  scraper controller, so it covers what the controller actually exports.
- `cmd/otelcol-fritzbox/main_test.go` runs the whole distribution against the
  fake device and an OTLP/HTTP sink, and validates every example config.

Model new device behavior (a missing service, a faulting action) by adjusting
the `fakebox.Box` fields in a test rather than by adding a real-device fixture.

## Metric schema changes

Metrics are declared in `receiver/fritzbox/metadata.yaml`. After editing,
regenerate the code and include the result in the same commit:

```sh
go generate ./receiver/...
git diff   # inspect the generated changes
```

CI fails if the generated code is stale.

## Container image

`make image-smoke` builds the image for the host platform and runs
`scripts/image-smoke.sh` (version, bundled config validation, non-root user,
health check). `make image-multiarch` builds every release platform. CI runs
both on pull requests.

## Collector dependencies

`go.mod` and `example/builder-config.yaml` must pin the same Collector
versions, and every component in the builder config must be registered in
`cmd/otelcol-fritzbox/components.go`. `make builder-check` enforces both.
Dependabot groups Collector updates but does not edit the builder config, so
update it in the same pull request.

## Commits and PRs

- Use [Conventional Commits](https://www.conventionalcommits.org/) with short,
  single-sentence messages (`feat:`, `fix:`, `docs:`, `chore:`, ...).
- Keep changes focused; one concern per PR.
- Tests must pass and new behavior needs a focused test. For bug fixes, add a
  regression test that fails on the old behavior where practical.
- No credentials, captured private device data, or build artifacts in commits.

## Issues and labels

Issues use forms (bug report, feature request, device report) that apply a
type label and `needs-triage`. The Label workflow then adds `area/*` and
`connection/*` labels from `.github/issue-labeler.yml` (regex on title and
body), and pull requests get `area/*` labels from `.github/labeler.yml`
(changed paths). Labels are only added, never removed, so maintainers can
correct them by hand.

## Device coverage

The support matrix in the README grows through user reports. If you tested a
device, open a device report with: model, FRITZ!OS version, connection type,
which metric groups worked, and any anomalies.

## Releases

Releases are cut by maintainers via version tags (`v*`), which triggers the
GoReleaser workflow. Do not push tags in PRs.
