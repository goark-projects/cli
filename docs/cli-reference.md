# CLI Command Reference

English | [简体中文](cli-reference.zh-CN.md)

## Syntax

```text
goark <command> [arguments]
```

Use `goark help`, `goark help <command>`, or `<command> --help` where supported. Diagnostics are written to standard error; structured and query results use standard output.

## Command Matrix

| Command | Reads `goark.build` | Writes project files | Starts processes | Purpose |
| --- | --- | --- | --- | --- |
| `run` | Yes | Generation/cache | Yes | Generate and run an application. |
| `build` | Yes | Generation/cache/output | Yes | Generate and execute `go build`. |
| `test` | Yes | Generation/cache | Yes | Generate and execute `go test`. |
| `install` | Yes | Generation/cache | Yes | Generate and execute `go install`. |
| `vet` | Yes | Generation/cache | Yes | Generate and execute `go vet`. |
| `list` | Yes | Generation/cache | Yes | Generate and execute `go list`. |
| `fix` | Yes | Source/generation/cache | Yes | Execute `go fix`, then regenerate. |
| `generate` | Yes | Generated source/cache | Tool tasks only | Run Goark-owned generation. |
| `clean` | Yes | Deletes declared outputs/cache | No | Remove declared outputs and task cache. |
| `tasks` | Yes | No | Metadata discovery only | List declared tasks. |
| `task` | Yes | Task-dependent | Task-dependent | Execute a named task and dependencies. |
| `graph` | Yes | No | Metadata discovery only | Render the validated task graph. |
| `sync` | Yes | Lock/trust/tool cache | Possibly | Resolve tools and update or verify the lock. |
| `tools` | Yes | No | No installation | Report tool state. |
| `tool` | Yes | Lock/trust/tool cache | Possibly | Install one tool or verify all tools. |
| `doctor` | Yes | No | Go version probe | Diagnose project, graph, Go, and tools. |
| `info` | Yes | No | Metadata discovery only | Show stable read-only diagnostics and plans. |
| `go` | No | Go-dependent | Yes | Execute the official Go command unchanged. |
| `new` | No | Yes | No | Create an `app` or `web` project. |
| `codegen` | No | Optional output | No child process | Run a low-level source generator. |
| `completion` | No | No | No | Print a shell completion script. |
| `help` / `version` | No | No | No | Print help or version information. |

Dry-run changes the write/process behavior of enhanced lifecycle commands and `clean` to read-only reporting.

## Common Goark Controls

Enhanced commands accept:

| Option | Meaning |
| --- | --- |
| `--goark-profile=<name>` | Select a Profile declared in `goark.build`. |
| `--goark-dry-run` | Print planned generation, tasks, and Go commands without writes or processes. |
| `--goark-offline` | Prohibit network access and automatic Go tool restoration. |
| `--goark-locked` | Require complete, exact current-platform lock entries and reject drift. |
| `--goark-env=KEY=VALUE` | Add a highest-priority environment override; repeatable. |

The removed `--goark-no-generate` and `--goark-generate-only` options fail. Use `goark go ...` for unmodified Go behavior.

## `run`

```text
goark run [go-build-flags] [package-or-go-files] [properties] [-- application-arguments]
```

```bash
goark run
goark run -race -tags=dev ./cmd/server
goark run ./cmd/server -Dserver.port=9090 --feature.enabled=true
goark run ./cmd/server -- --job=sync input.json
```

When the target is omitted, Goark uses `project.main` or discovers a unique main package. `-Dkey=value` and `--key=value` before the separator are forwarded as Boot properties. Everything after `--` is an ordinary application argument.

## Enhanced Go Commands

```text
goark build [go-build-arguments]
goark test [go-test-arguments]
goark install [go-install-arguments]
goark vet [go-vet-arguments]
goark list [go-list-arguments]
goark fix [go-fix-arguments]
```

All ordinary arguments are passed to the matching official Go subcommand after configured and Profile arguments. Go's global `-C` is moved before the subcommand as required by official syntax.

```bash
goark build
goark build -o ./build/custom ./cmd/server
goark test -race -count=1 ./...
goark vet ./...
goark list -deps ./...
goark install ./cmd/...
goark fix ./...
goark build -C services/admin ./...
```

For `build`, configured `commands.build.output` becomes `-o <path>` only when the CLI does not already contain an output flag. When no build target is supplied and `project.main` is configured, that main package is used.

## `generate`

```text
goark generate [package-patterns] [Go-loading-flags] [--goark-*]
```

```bash
goark generate
goark generate ./internal/...
goark generate -tags=integration ./...
goark generate --goark-profile=production
```

Patterns default to `generate.patterns`, which defaults to `./...`. Supported discovery flags include `-C`, `-tags`, `-mod`, `-modfile`, and `-overlay`. This command does not execute `go generate`.

## `clean`

```text
goark clean [--goark-dry-run]
```

Removes every declared command `output`, every task `outputs` match, and the project task cache under `.goark/cache`. Paths are resolved inside the project. Preview first in important worktrees:

```bash
goark clean --goark-dry-run
goark clean
```

## Task Commands

### `tasks`

```text
goark tasks [--json]
```

Lists declared task metadata. `--json` provides stable machine-readable output.

### `task`

```text
goark task <name> [--goark-profile=<name>] [--goark-dry-run]
```

Executes the target and all upstream dependencies. Common offline, locked, and environment controls are also parsed when supplied.

```bash
goark task orm-generate
goark task release --goark-profile=production --goark-locked
```

### `graph`

```text
goark graph [--format=text|json|dot]
```

The default format is `text`. Use JSON for tools and DOT for Graphviz:

```bash
goark graph
goark graph --format=json
goark graph --format=dot > tasks.dot
```

## Tool Commands

### `sync`

```text
goark sync [--locked] [--offline]
```

- With no options, resolves all tools, may auto-install eligible Go tools, writes `goark.build.lock`, and records project trust.
- `--locked` performs verification only and does not update the lock.
- `--offline` forbids network access and installation but may update lock/trust from locally resolvable tools.

### `tools`

```text
goark tools [--json]
```

Reports each declared tool as `ready`, `missing`, `unlocked`, `drift`, or `error` without installing it.

### `tool`

```text
goark tool install <name>
goark tool verify
```

`install` explicitly resolves or installs one declared tool and updates its current-platform lock entry. `verify` checks all declarations, lock entries, and executable digests without installation.

## Diagnostics

### `doctor`

```text
goark doctor
```

Checks `goark.build`, the task graph, Go toolchain availability, and each tool. It returns nonzero when any check fails.

### `info`

```text
goark info [--goark-profile=<name>] [--json]
```

`info` is read-only. It reports CLI/Go metadata, project identity, selected Profile, main package, tool status, tasks, generators, cache statistics, and final plans for all enhanced commands. It does not generate source, install tools, update trust, or modify the lock.

## Official Go Proxy

```text
goark go <go-arguments>
```

All arguments, standard streams, signals, environment, and available exit codes are passed through without loading `goark.build` or running generation.

```bash
goark go version
goark go env GOMOD
goark go generate ./...
goark go test ./...
```

## Project Creation

```text
goark new [-type app|web] [-module <module-path>] [-dir <path>] <name>
```

See the [application creation guide](guides/project-creation.md).

## Low-Level Code Generation

```text
goark codegen configuration --name <name> --package <package> [flags]
goark codegen registry --package <package> --configuration <type> [flags]
goark codegen annotations --dir <package-dir> [flags]
```

Low-level generators write to standard output unless `--output` is provided. Repeated `--import`, `--bean`, and `--configuration` options build explicit declarations. Use `goark help codegen <generator>` for the complete option grammar. Normal projects should prefer `goark generate`.

## Completion, Help, and Version

```bash
goark completion bash
goark completion zsh
goark completion fish
goark completion powershell
goark help [command]
goark version
```

Install completion for the current session:

```bash
source <(goark completion bash)
```

```powershell
goark completion powershell | Out-String | Invoke-Expression
```

## Exit Codes

| Code | Meaning |
| --- | --- |
| `0` | Success. |
| `1` | Goark validation/execution failure without a more specific child code. |
| `2` | CLI usage, configuration, or project-resolution error. |
| `130` | Cancellation/interruption when no child code is available. |
| Other | Preserved child process exit code when available. |
