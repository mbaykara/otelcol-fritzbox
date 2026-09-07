# Contributing

## Getting started

Requires Go 1.27.

```sh
go test ./...
go vet ./...
go build ./cmd/otelcol-fritzbox
```

For concurrency or lifecycle changes also run `go test -race -count=1 ./receiver/...`.

## Metric schema changes

Metrics are declared in `receiver/fritzbox/metadata.yaml`. After editing,
regenerate the code and include the result in the same commit:

```sh
go generate ./receiver/...
git diff   # inspect the generated changes
```

CI fails if the generated code is stale.

## Commits and PRs

- Use [Conventional Commits](https://www.conventionalcommits.org/) with short,
  single-sentence messages (`feat:`, `fix:`, `docs:`, `chore:`, ...).
- Keep changes focused; one concern per PR.
- Tests must pass and new behavior needs a focused test. For bug fixes, add a
  regression test that fails on the old behavior where practical.
- No credentials, captured private device data, or build artifacts in commits.

## Device coverage

The support matrix in the README grows through user reports. If you tested a
device, open an issue or PR with: model, FRITZ!OS version, connection type,
which metric groups worked, and any anomalies.

## Releases

Releases are cut by maintainers via version tags (`v*`), which triggers the
GoReleaser workflow. Do not push tags in PRs.
