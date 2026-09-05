# `goark.build` Reference

English | [简体中文](goark-build.zh-CN.md)

`goark.build` is the only Goark project description file. It controls build-time orchestration: project entry points, generated code, command hooks, external tools, tasks, Profiles, environments, concurrency, and caching. It does not replace `go.mod` or runtime application configuration.

## File Contract

| Property | Requirement |
| --- | --- |
| File name | Exactly `goark.build` |
| Location | Same directory as `go.mod` |
| Syntax | TOML 1.0 with `#` comments |
| Encoding | Valid UTF-8 without BOM |
| Line endings | LF only; CRLF and CR are rejected |
| Required field | Integer `version = 1` |
| Parsing | Unknown and duplicate fields are rejected |

Module path, Go language version, and `toolchain` are read only from `go.mod` and must not be duplicated here.

## Minimal Configuration

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

Only `version` is required. This small form is recommended until a project actually needs hooks, external tools, caching, or Profiles.

## Top-Level Sections

| Section | Required | Purpose |
| --- | --- | --- |
| `version` | Yes | Selects the file format; only integer `1` is supported. |
| `[project]` | No | Human project metadata and the default main package. |
| `[execution]` | No | Global task scheduler and timeout policy. |
| `[generate]` | No | Goark-owned compile-time generation scope. |
| `[commands.<name>]` | No | Hooks, arguments, environment, and output for a fixed command. |
| `[tools.<name>]` | No | External executable declarations. |
| `[tasks.<name>]` | No | Task graph nodes. |
| `[profiles.<name>]` | No | Named build-time argument and environment overlays. |

## `[project]`

```toml
[project]
name = "admin"
main = "./cmd/server"
description = "Administration service"
```

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `name` | string | Last segment of the `go.mod` module path | Display name and `${project.name}` value. |
| `main` | project-relative string | Auto-discovered | Default main package for `run` and target for `build` when no target is supplied. |
| `description` | string | Empty | Human-readable metadata reported by `info`. |

Main discovery uses the current package when it is `main`, otherwise a unique main package under `./cmd/...`. Multiple candidates require an explicit target.

## `[execution]`

```toml
[execution]
max-parallel = 4
fail-fast = true
default-timeout = "5m"
```

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `max-parallel` | positive integer | Current `GOMAXPROCS`, at least 1 | Maximum number of ready task nodes executed concurrently. |
| `fail-fast` | boolean | `true` | Cancels remaining schedulable work after the first task failure. |
| `default-timeout` | Go duration string | `5m` | Timeout used by tasks that do not declare `timeout`; also bounds `finally` processing. |

Duration values use Go syntax such as `500ms`, `30s`, `2m`, or `1h30m` and must be positive here.

## `[generate]`

```toml
[generate]
patterns = ["./..."]
clean-stale = true
```

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `patterns` | string array | `["./..."]` | Project-local Go package patterns scanned by built-in generators. |
| `clean-stale` | boolean | `true` | Removes obsolete files only when they carry the Goark generated-file ownership header. |

Patterns must be `.` or begin with `./` and must remain inside the project. Generation respects relevant Go loading flags such as `-tags`, `-mod`, `-modfile`, and `-overlay`.

## `[commands.<name>]`

Supported names are `generate`, `run`, `build`, `test`, `install`, `vet`, `list`, and `fix`.

```toml
[commands.build]
before = ["assets"]
after = ["checksum"]
finally = ["cleanup-temp"]
go-args = ["-trimpath"]
application-args = []
output = "./build/admin"

[commands.build.environment]
CGO_ENABLED = "0"
```

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `before` | task-name array | Empty | Tasks run before the official command, after generation. For `fix`, before `go fix`. |
| `after` | task-name array | Empty | Tasks run only when the main lifecycle succeeds. |
| `finally` | task-name array | Empty | Tasks run in reverse dependency order whether the main lifecycle succeeds or fails. |
| `go-args` | string array | Empty | Arguments prepended before Profile and CLI Go arguments. |
| `application-args` | string array | Empty | Default application arguments, primarily useful for `run`. |
| `environment` | string map | Empty | Environment overlay for this command. |
| `output` | project-relative string | Empty | Available as `${command.output}`; for `build`, applied as `go build -o` unless the CLI already supplies `-o`. |

Every hook name must reference a declared task. Empty sections and empty arrays are optional; do not add them only for ceremony.

## `[tools.<name>]`

Tool and task names must match `[A-Za-z0-9][A-Za-z0-9._-]*`.

### Go Tool

```toml
[tools.goark-orm]
type = "go"
package = "goark.dev/orm/cmd/goark-orm"
version = "v0.1.0"
install = "auto"
```

- `package` and an exact semantic `version` are required.
- `install` is `auto` or `manual`.
- The executable is installed into Goark's user cache, not the project or global `GOBIN`.
- Automatic restoration requires a matching lock file and trusted project digest.

### System Tool

```toml
[tools.sha256]
type = "system"
command = "sha256sum"
install = "manual"
```

- `command` is a command name resolved from `PATH`, not a path or shell expression.
- System tools are never installed by Goark.
- `install` must be `manual`.

### Local Tool

```toml
[tools.asset-compiler]
type = "local"
path = "./tools/asset-compiler"
install = "manual"
```

- `path` must resolve to an executable inside the project after symlink resolution.
- `install` must be `manual`.

Fields from one tool type cannot be mixed with another. See [tools and locking](tools-lock-cache.md) for lifecycle details.

## `[tasks.<name>]`

```toml
[tasks.orm-generate]
type = "exec"
tool = "goark-orm"
args = ["generate", "orm", "./..."]
working-directory = "."
depends-on = []
inputs = ["**/*.go", "resource/orm/**/*.xml"]
outputs = ["**/zz_goark_orm_*_gen.go"]
environment-inputs = ["GOARK_ORM_CONFIG"]
timeout = "2m"
cache = true
parallel-safe = false
when = 'profile != "docs"'

[tasks.orm-generate.environment]
GOARK_ORM_CONFIG = "resource/orm/config.toml"
```

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `type` | enum | Required | `exec`, `go`, `delete`, or `group`; `goark-generate` is CLI-internal and rejected in user files. |
| `tool` | string | Empty | Required only by `exec`; references `[tools.<name>]`. |
| `args` | string array | Empty | Argument array passed without a shell. A `go` task must declare at least one argument. |
| `working-directory` | project-relative string | Project root | Existing directory used for process execution. |
| `depends-on` | task-name array | Empty | Upstream tasks that must complete first. |
| `inputs` | path/glob array | Empty | Files and directories included in a cache fingerprint. |
| `outputs` | path/glob array | Empty | Declared output ownership, conflict checks, cache validation, and delete targets. |
| `environment-inputs` | environment-name array | Empty | Environment values hashed into the cache key without storing plaintext. |
| `environment` | string map | Empty | Task-local environment overlay; values support substitutions. |
| `timeout` | Go duration string | `execution.default-timeout` | Per-task timeout. `0` means use the global default; negative values are invalid. |
| `cache` | boolean | `false` | Enables output-verified task caching; requires non-empty `inputs` and `outputs`. |
| `parallel-safe` | boolean | `false` | Opts the task into concurrent execution with other safe ready nodes. |
| `when` | expression string | Always true | Side-effect-free condition evaluated before execution. |

### Task Types

| Type | Behavior | Important rules |
| --- | --- | --- |
| `exec` | Executes a declared external tool. | `tool` must exist; executable and args remain separate. |
| `go` | Executes the official `go` executable with `args`. | First argument is normally a Go subcommand. |
| `delete` | Deletes matching declared outputs. | Outputs must remain inside the project; caching is forbidden. |
| `group` | Aggregates dependencies without starting a process. | Must declare at least one dependency. |

## `[profiles.<name>]`

```toml
[profiles.production]
go-args = ["-tags=production", "-trimpath"]
application-args = []

[profiles.production.environment]
GOARK_PROFILES_ACTIVE = "production"
```

| Field | Type | Default | Meaning |
| --- | --- | --- | --- |
| `go-args` | string array | Empty | Build arguments inserted after command defaults and before CLI arguments. |
| `application-args` | string array | Empty | Application arguments inserted after command defaults and before CLI arguments. |
| `environment` | string map | Empty | Environment overlay inserted above command environment. |

Select a Profile with `--goark-profile=production`. An unknown Profile fails. This is a build-time selection and is not inferred from the Boot runtime property `goark.profiles.active`.

## Environment and Argument Precedence

For environment values, highest priority wins:

```text
--goark-env
Profile environment
command environment
task environment
process environment
```

For Go and application argument arrays, values are assembled in this order:

```text
command values -> Profile values -> CLI values
```

This preserves the CLI as the final layer for Go flags whose last occurrence wins.

Environment variable names must match `[A-Za-z_][A-Za-z0-9_]*`. On Windows, names are compared case-insensitively.

## Variable Substitution

Substitution is performed once in task `args`, `inputs`, `outputs`, `working-directory`, and task environment values.

| Variable | Value |
| --- | --- |
| `${project.root}` | Canonical project root |
| `${project.name}` | Effective project name |
| `${project.module}` | Module path from `go.mod` |
| `${profile}` | Selected build Profile, or empty |
| `${command.output}` | Current command `output`; fails when unavailable |
| `${tool:name}` | Verified executable path for a declared tool |
| `${env:NAME}` | Effective declared/process environment value |

Substitution is non-recursive. Backticks, `$()`, unknown variables, missing environment values, and missing tools fail. No Shell expansion is performed.

## `when` Expressions

Allowed values are `profile`, `goos`, `goarch`, `env.NAME`, string literals, `true`, and `false`. Allowed operators are `==`, `!=`, `&&`, `||`, `!`, and parentheses.

```toml
when = 'profile == "production" && goos == "linux"'
```

An `env.NAME` reference must exist in the effective environment. Function calls, commands, numbers, and arbitrary identifiers are rejected.

## Complete Example

```toml
version = 1

[project]
name = "admin"
main = "./cmd/server"
description = "Administration service"

[execution]
max-parallel = 4
fail-fast = true
default-timeout = "5m"

[generate]
patterns = ["./..."]
clean-stale = true

[commands.generate]
before = ["orm-generate"]

[commands.build]
after = ["checksum"]
go-args = ["-trimpath"]
output = "./build/admin"

[commands.test]
go-args = ["-count=1", "./..."]

[tools.goark-orm]
type = "go"
package = "goark.dev/orm/cmd/goark-orm"
version = "v0.1.0"
install = "auto"

[tools.sha256]
type = "system"
command = "sha256sum"
install = "manual"

[tasks.orm-generate]
type = "exec"
tool = "goark-orm"
args = ["generate", "orm", "./..."]
inputs = ["**/*.go", "resource/orm/**/*.xml"]
outputs = ["**/zz_goark_orm_*_gen.go"]
environment-inputs = ["GOARK_ORM_CONFIG"]
cache = true

[tasks.orm-generate.environment]
GOARK_ORM_CONFIG = "resource/orm/config.toml"

[tasks.checksum]
type = "exec"
tool = "sha256"
args = ["${command.output}"]

[profiles.dev]
go-args = ["-tags=dev"]

[profiles.dev.environment]
GOARK_PROFILES_ACTIVE = "dev"

[profiles.production]
go-args = ["-tags=production", "-trimpath"]

[profiles.production.environment]
GOARK_PROFILES_ACTIVE = "production"
```

The `sha256sum` example is Linux-specific. Declare a different system tool or task condition for Windows when the project must be cross-platform.

## Validation and Safety

Before execution, Goark rejects invalid TOML, unsupported versions, unknown fields, invalid identifiers, project path escapes, symlink escapes, missing dependencies, duplicate dependencies, cycles, overlapping task outputs, missing tools, and lock drift.

Secrets whose environment names contain markers such as `PASSWORD`, `PASSWD`, `SECRET`, `TOKEN`, `API_KEY`, `PRIVATE_KEY`, or `CREDENTIAL` are redacted in plans and diagnostic output.
