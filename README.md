# Goark CLI

<p align="center">
  <img src="assets/goark-readme-logo.png" alt="Goark" width="520">
</p>

<p align="center">
  Deterministic project orchestration for Goark applications, built on the official Go toolchain.
</p>

<p align="center">
  English | <a href="README.zh-CN.md">简体中文</a>
</p>

`goark` is the command-line entry point for the Goark ecosystem. A project is described by one strict TOML file, `goark.build`. The CLI combines Goark-owned code generation, lifecycle tasks, external tool locking, task caching, Profiles, and normal Go commands without replacing the Go toolchain.

## Install

Go 1.25 or later is required.

```bash
go install goark.dev/cli/cmd/goark@latest
goark version
```

## Create a Project

Create a non-Web Boot application in the current directory:

```bash
mkdir worker && cd worker
goark new worker
go mod tidy
goark run
```

Create a Web application in a named directory:

```bash
goark new -type web -module example.com/admin -dir admin admin
cd admin
go mod tidy
goark run
```

`-type` defaults to `app`; `-module` defaults to the project name; `-dir` defaults to the current directory. Both templates include Goark Boot, configuration, dependency injection, and `goark.dev/gbc-log`. The `web` template additionally includes Arkarta, Arkhos, HTTP auto-configuration, a health endpoint, and static resources.

## Everyday Workflow

```bash
# Inspect the project and final execution plans without writing files.
goark info
goark info --json

# Resolve tools and create or update goark.build.lock.
goark sync

# Generate Goark code, then call the official Go command.
goark run
goark build
goark test ./...

# Preview the lifecycle without processes or writes.
goark build --goark-dry-run

# Bypass Goark orchestration and run Go directly.
goark go generate ./...
goark go test ./...
```

Every project-aware command requires `goark.build` next to `go.mod`. The module path, Go language version, and toolchain remain owned exclusively by `go.mod`.

## Minimal `goark.build`

```toml
version = 1

[project]
name = "acb"
main = "./cmd/server"

[commands.build]
go-args = ["-trimpath"]
output = "./build/acb"

[commands.test]
go-args = ["./..."]
```

All sections except `version` are optional and have safe defaults. Unknown fields, duplicate fields, invalid paths, task cycles, missing tools, and lock drift fail before the main command starts.

## Documentation

- [Documentation index](docs/README.md)
- [Getting started](docs/getting-started.md)
- [`goark.build` reference](docs/goark-build.md)
- [CLI command reference](docs/cli-reference.md)
- [Code generation](docs/code-generation.md)
- [Lifecycle and task graph](docs/lifecycle-and-tasks.md)
- [Tools, lock file, trust, and cache](docs/tools-lock-cache.md)
- [Application creation guide](docs/guides/project-creation.md)
- [CI and offline workflows](docs/guides/ci-workflows.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Versioning and releases](docs/versioning-and-releases.md)
- [Changelog](CHANGELOG.md)

## Design Boundaries

- `goark.build` is the only Goark project description file.
- `goark go ...` is a transparent route to the official Go command.
- `goark generate` runs Goark compile-time generators; it does not run `go generate`.
- Enhanced `run`, `build`, `test`, `install`, `vet`, and `list` commands generate first.
- `fix` runs `go fix` first and regenerates after success.
- External commands use an executable plus an argument array, never a shell string.
- Generated project files use UTF-8 without BOM and LF line endings.
- Agents and Plugins are outside the `goark.build` V1 contract.

## Development

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go run ./cmd/goark --help
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
