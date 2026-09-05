# Getting Started

English | [简体中文](getting-started.zh-CN.md)

## Prerequisites

- Go 1.25 or later.
- A supported Windows, Linux, or macOS environment.
- Git and network access when a declared Go tool must be installed.

Install the CLI:

```bash
go install goark.dev/cli/cmd/goark@latest
```

Ensure the Go binary directory is on `PATH`, then verify:

```bash
goark version
goark help
```

## Start with a New Application

The shortest form creates an `app` project in the current directory. The project name is also used as the module path when `-module` is omitted.

```bash
mkdir hello
cd hello
goark new hello
```

For a Web service, normally provide a globally unique module path:

```bash
goark new -type web -module example.com/team/hello -dir hello hello
cd hello
```

Generated entry points are deliberately small:

```go
package main

import "os"

func main() {
	os.Exit(runGoark(os.Args[1:]))
}
```

The adjacent `goark.go` owns Boot startup, argument forwarding, auto-configuration, signal handling where required, shutdown, and the process exit code.

## Generated Layout

An `app` project contains:

```text
.
|-- go.mod
|-- goark.build
|-- resource/app.yml
|-- cmd/app/main.go
|-- cmd/app/goark.go
`-- internal/app/configuration.go
```

A `web` project uses `cmd/server`, adds `resource/static/index.html`, and registers the Arkhos server, MVC routes, HTTP client customization, and `GET /healthz`.

Both templates include `goark.dev/gbc-log` and register its auto-configuration.

## Prepare and Inspect

Resolve module dependencies first:

```bash
go mod tidy
```

`goark info` is the safest first project command. It validates the project and reports the selected Profile, Go metadata, main package, tools, tasks, generator scan, cache, and final plans without installing tools or generating code.

```bash
goark info
goark info --json
```

If `goark.build` declares tools, synchronize them before running a lifecycle:

```bash
goark sync
goark tool verify
```

Commit `goark.build.lock` so CI and other developers can verify identical tool identities.

## Run, Build, and Test

```bash
goark run
goark build
goark test ./...
goark vet ./...
```

These enhanced commands validate `goark.build`, select a build Profile, resolve required locked tools, run Goark generation, execute lifecycle tasks, and finally call the official Go command.

Preview without side effects:

```bash
goark run --goark-dry-run
goark build --goark-dry-run
```

Use the official Go command unchanged when orchestration is not wanted:

```bash
goark go env GOMOD
goark go generate ./...
goark go test ./...
```

## Runtime Arguments

`goark run` keeps four categories separate:

```bash
goark run -race -tags=dev ./cmd/server \
  -Dserver.port=9090 \
  --goark.profiles.active=dev \
  -- --job=sync input.json
```

| Segment | Destination |
| --- | --- |
| `-race -tags=dev` | Go build flags |
| `./cmd/server` | Main package |
| `-Dserver.port=9090` | Boot system property |
| `--goark.profiles.active=dev` | Boot command-line property |
| Values after `--` | Ordinary application arguments |

The build Profile selected by `--goark-profile=dev` is independent from the Boot runtime property `--goark.profiles.active=dev`. Configure both when both build-time and runtime behavior must change.

## Next Steps

- Read the complete [`goark.build` reference](goark-build.md).
- Learn the [fixed lifecycle and task graph](lifecycle-and-tasks.md).
- Browse the [CLI command reference](cli-reference.md).
