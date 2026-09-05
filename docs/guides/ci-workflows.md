# CI and Offline Workflows

English | [简体中文](ci-workflows.zh-CN.md)

## Repository Files

Commit `go.mod`, `go.sum`, `goark.build`, and `goark.build.lock` when tools or locked execution are used. Commit Goark-generated `.go` files when repository policy tracks generated source. Do not commit `.goark/cache`, the user tool cache, or project trust records.

## Developer Preparation

After changing `goark.build`, synchronize and verify locally:

```bash
goark sync
goark doctor
goark info --json
goark test -race ./...
goark vet ./...
```

Run `goark sync` on every supported GOOS/GOARCH when the lock must carry multiple platform entries.

## Locked CI

```bash
goark sync --locked
goark generate --goark-locked
goark test --goark-locked -count=1 ./...
goark vet --goark-locked ./...
goark build --goark-locked
```

`--goark-locked` requires complete lock entries for all declared tools on the current platform, including tools not reached by the selected command. This is the strictest reproducibility gate.

## GitHub Actions Example

```yaml
name: validation

on:
  push:
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - run: go install goark.dev/cli/cmd/goark@latest
      - run: goark sync --locked
      - run: goark test --goark-locked -race -count=1 ./...
      - run: goark vet --goark-locked ./...
      - run: goark build --goark-locked
```

The general example tracks `@latest`. For a reproducible release pipeline, replace it with the exact approved tag, such as `@v0.0.1`.

## Generated Source Drift

When generated files are committed, regenerate and require a clean diff:

```bash
goark generate --goark-locked
git diff --exit-code -- '*.go'
```

Every generation rebuilds and atomically replaces Goark-owned targets, so this detects stale or non-deterministic output.

## Offline Execution

```bash
goark tool verify
goark test --goark-offline --goark-locked ./...
goark build --goark-offline --goark-locked
```

Prepare the Go module cache and Goark tool cache before disconnecting. System and local tools must already be available.

`goark sync --offline` can update a lock from locally resolvable tools; it is not read-only. Use `goark sync --locked` or `goark tool verify` for verification without lock updates.

## Profiles in CI

Build Profiles and Boot runtime Profiles are independent:

```bash
goark build --goark-profile=production --goark-locked
goark run --goark-profile=production --goark.profiles.active=production
```

Use `--goark-profile` for `goark.build` Go arguments, task conditions, and environment overlays. Use `GOARK_PROFILES_ACTIVE` or `--goark.profiles.active` for Boot configuration files and runtime beans.

## Cross-Platform Rules

- Avoid unconditional `system` tools that exist on only one OS.
- Use a platform `when` condition or separate platform tasks where appropriate.
- A condition skips task execution, but a reachable `exec` task's tool may still be required during lifecycle preparation.
- Prefer Go tools for portable behavior when an equivalent package exists.
- Generate and commit lock entries from each supported platform.

## Dry-Run Gate

```bash
goark info --json > goark-info.json
goark build --goark-profile=production --goark-dry-run
```

Dry-run validates configuration, paths, graph, lock requirements, and plans without writes or processes. A reachable external tool still requires a valid lock and locally resolvable verified executable so its final path can be reported.
