# goark.build V1 Specification

## Scope

`goark.build` is the only Goark project description file. It defines the project entry point, fixed command lifecycles, external tools, task graph, Profiles, environment overrides, caching, and execution policy. V1 does not include Agents, Plugins, runtime scanning, a shell interpreter, or a replacement for the Go toolchain.

The module path, Go language version, and toolchain are read only from the adjacent `go.mod`.

## File Contract

- File name: `goark.build`.
- Location: the directory containing `go.mod`.
- Encoding: UTF-8 without BOM.
- Line endings: LF.
- Syntax: strict TOML 1.0 with `#` comments.
- `version = 1` is required.
- Unknown or duplicate fields, invalid enums, and invalid references fail before a child process starts.

Project-aware commands fail when the file is missing:

```text
run build test install vet list fix generate clean
tasks task graph sync tools tool doctor info
```

These commands do not read it:

```text
go new codegen version help completion
```

## Minimal Example

```toml
version = 1

[project]
name = "admin-minimal"
main = "./cmd/admin"

[execution]
max-parallel = 4
fail-fast = true
default-timeout = "5m"

[generate]
patterns = ["./..."]
clean-stale = true

[commands.build]
go-args = ["-trimpath"]
output = "./build/admin-minimal"

[commands.test]
go-args = ["-count=1", "./..."]
```

## Fixed Lifecycle

Enhanced commands execute this order:

```text
load and validate goark.build
select Profile
merge arguments and environment
resolve tools and verify goark.build.lock
build and validate the task DAG
commands.generate.before
Goark built-in generation
commands.generate.after
current command before
official Go command
current command after, only after success
current command finally, always
```

`run`, `build`, `test`, `install`, `vet`, and `list` always generate first. `fix` runs `go fix` first and regenerates after success. `generate` runs only the Goark generation lifecycle and never invokes official `go generate`; use `goark go generate` for that behavior.

The lifecycle cannot be bypassed. The removed `--goark-no-generate` and `--goark-generate-only` flags are rejected.

## Arguments, Profiles, and Environment

Build-plan precedence, from highest to lowest:

```text
CLI arguments and --goark-env
selected Profile
commands.<name>
process environment
tool defaults
```

Supported controls are:

```text
--goark-profile=<name>
--goark-dry-run
--goark-offline
--goark-locked
--goark-env=KEY=VALUE
```

`run` stores Go arguments, application properties, and application arguments separately. `-Dkey=value` and properties after the main package are application properties. Everything after `--` is an ordinary application argument.

```bash
goark run -race ./cmd/admin \
  -Dserver.port=9090 \
  --goark.profiles.active=dev \
  -- --job=sync input.json
```

Only these substitutions are allowed:

```text
${project.root}
${project.name}
${project.module}
${profile}
${command.output}
${tool:<name>}
${env:<NAME>}
```

Backticks, `$()`, shell expansion, recursive substitution, and command strings are forbidden.

## Tools and Lock File

Tools have one of three sources:

- `go`: installed at an exact module version into Goark's isolated cache.
- `system`: resolved from `PATH` and never installed automatically.
- `local`: an executable inside the project boundary.

`goark sync` atomically creates or updates `goark.build.lock`. Go tools are locked by package, version, module checksum, platform, and executable digest. System and local tools are locked by canonical path, platform, and executable digest. Automatic restoration requires a complete lock and a trust record matching the canonical project root and current `goark.build` digest.

Use `--locked` to prohibit lock updates and `--offline` to prohibit network access and installation.

## Tasks and Concurrency

User declarations support `exec`, `go`, `delete`, and `group`. `goark-generate` is internal to the CLI. Common fields include `depends-on`, `working-directory`, `args`, `environment`, `timeout`, `inputs`, `outputs`, `cache`, `parallel-safe`, and `when`.

Before execution, Goark rejects cycles, missing tasks, duplicate dependencies, output conflicts, invalid paths, symlink escapes, missing tools, and lock drift. Tasks run concurrently only when all ready tasks declare `parallel-safe = true` and their outputs do not overlap. Shutdown and `finally` processing use reverse dependency order.

## Cache

A task fingerprint covers the normalized task definition, input content, locked tool data, Go version, GOOS, GOARCH, build tags, Profile, declared environment inputs, and upstream output digests. A hit requires every output to exist with the recorded digest. Cache publication uses a temporary directory and atomic replacement under the cross-process project lock.

Cached `exec` tasks must explicitly set `cache = true` and declare both `inputs` and `outputs`.

## Generated File Ownership

Every generation run rebuilds complete source and atomically replaces each owned target through a temporary file in the same directory. An existing target must start with:

```go
// Code generated by goark; DO NOT EDIT.
```

Goark refuses to overwrite a same-named handwritten file. With `clean-stale = true`, it deletes only files carrying the ownership header that no longer correspond to an annotated package. Dry-run never creates, replaces, or deletes files.

## Security and Process Behavior

- External commands are always an executable plus an argument array, never a shell string.
- Inputs, outputs, working directories, and local tools remain inside the canonical project root after symlink resolution.
- Standard input, output, error, signals, and child exit codes are preserved where the operating system permits.
- Cancellation terminates the task graph and child process tree.
- Secret environment names are redacted in logs, JSON, and dry-run output.
- `info` is read-only and reports only declared environment overrides, not the full process environment.
- Dry-run starts no process and writes no project, cache, trust, tool, or lock state.

## CLI Surface

```text
goark run/build/test/install/vet/list/fix/generate
goark clean
goark tasks [--json]
goark task <name>
goark graph [--format=text|json|dot]
goark sync [--locked] [--offline]
goark tools
goark tool install <name>
goark tool verify
goark doctor
goark info [--goark-profile=<name>] [--json]
goark go ...
goark new/codegen/version/help/completion
```

`goark info` is a read-only human report. `--goark-profile` selects the Profile used to construct its plans. `--json` emits a stable structure containing project metadata, Profile, tool status, tasks, generators, cache state, and final execution plans without installing tools or generating code.
