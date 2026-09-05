# Lifecycle and Task Graph

English | [简体中文](lifecycle-and-tasks.zh-CN.md)

## Why the Lifecycle Is Fixed

Goark treats generation and declared hooks as part of the build contract. A fixed order prevents local runs, CI builds, and releases from silently using different generated source or tool versions. There are no flags to skip generation inside an enhanced command; use `goark go ...` when the raw Go behavior is required.

## Normal Enhanced Commands

`run`, `build`, `test`, `install`, `vet`, and `list` use this order:

```text
load go.mod and strict goark.build
select --goark-profile
merge command, Profile, CLI, and environment values
validate the task DAG and project paths
resolve and verify required locked tools
acquire the cross-process project lock
  commands.generate.before
  Goark built-in code generation
  commands.generate.after
  commands.generate.finally
  commands.<current>.before
  official Go command
  commands.<current>.after, only after success
  commands.<current>.finally, always
release the project lock
```

The current implementation holds the project lock across the complete lifecycle. This prevents tasks, generated outputs, the official command, and finalization from observing concurrent mutations by another Goark lifecycle in the same project.

## `generate`

`goark generate` runs only:

```text
commands.generate.before
Goark built-in generation
commands.generate.after, only after success
commands.generate.finally, always
```

It never invokes `go generate`. Use `goark go generate ./...` for standard Go directives.

## `fix`

`goark fix` is deliberately different:

```text
commands.fix.before
go fix
complete generate lifecycle
commands.fix.after, only after success
commands.fix.finally, always
```

This ordering regenerates source from the code after `go fix` has modified it.

## Hook Failure Rules

- A failed `before`, built-in generation, or official Go command prevents `after`.
- `finally` always runs, including after cancellation or an earlier failure.
- `finally` uses reverse dependency order and continues after an individual finalizer fails.
- An earlier main failure keeps its exit code. A `finally` failure becomes the result only when the main lifecycle succeeded.
- Child process exit codes are preserved when the operating system exposes them. Cancellation maps to exit code 130.

## Task DAG

Each `[tasks.<name>]` entry is a node. `depends-on` points to upstream nodes:

```toml
[tasks.assets]
type = "go"
args = ["run", "./cmd/assets"]
outputs = ["internal/assets/assets_gen.go"]

[tasks.package]
type = "group"
depends-on = ["assets", "metadata"]
```

Before execution, Goark validates:

- Every target and dependency exists.
- A task does not repeat the same dependency.
- The graph has no cycles.
- Declared output paths do not overlap across tasks.
- Working directories, inputs, outputs, and local tools remain inside the canonical project root after symlink resolution.
- Every `exec` task references a declared tool.

## Task Types

### `exec`

Runs a declared tool as an executable plus an argument array:

```toml
[tools.formatter]
type = "system"
command = "gofmt"
install = "manual"

[tasks.format]
type = "exec"
tool = "formatter"
args = ["-w", "./internal"]
```

No shell is involved. Pipes, redirection, `&&`, command substitution, and shell built-ins are not interpreted.

### `go`

Runs the official Go executable with controlled arguments:

```toml
[tasks.verify-modules]
type = "go"
args = ["mod", "verify"]
```

### `delete`

Deletes only declared project-local output matches:

```toml
[tasks.clean-assets]
type = "delete"
outputs = ["internal/assets/*_gen.go"]
```

The project root itself cannot be deleted. Symlink targets are revalidated. Delete tasks cannot be cached.

### `group`

Aggregates one or more dependencies and starts no process:

```toml
[tasks.verify]
type = "group"
depends-on = ["unit-test", "static-check"]
```

`goark-generate` is reserved for the CLI's internal node and cannot be declared in `goark.build`.

## Concurrency

The scheduler uses `execution.max-parallel` as an upper bound. A ready task may run concurrently only when it declares `parallel-safe = true`; undeclared safety defaults to serial execution. Static output-conflict validation rejects tasks whose output paths or glob prefixes may overlap.

```toml
[execution]
max-parallel = 4

[tasks.unit]
type = "go"
args = ["test", "./internal/unit/..."]
parallel-safe = true

[tasks.integration]
type = "go"
args = ["test", "./internal/integration/..."]
parallel-safe = true
```

Declare `parallel-safe = true` only when the tool, working directory, environment, and undeclared side effects are also concurrency-safe. Disjoint `outputs` alone cannot prove that an external process is safe.

With `fail-fast = true`, the first failure cancels remaining task work. With `false`, independent ready work may finish and the executor still returns a failure.

## Conditions

`when` is evaluated immediately before a task starts:

```toml
when = 'profile == "production" && goos == "linux" && env.CI == "true"'
```

Skipped tasks are successful for dependency scheduling and contribute a stable `skipped` upstream state to downstream cache fingerprints.

## Generated File Ownership

Project generation writes `zz_goark_<package>_gen.go`. Every run rebuilds the complete content and atomically replaces an existing Goark-owned target. Ownership is identified by:

```go
// Code generated by goark; DO NOT EDIT.
```

Goark refuses to overwrite a same-named handwritten file. When `clean-stale = true`, only obsolete files carrying this exact ownership header are removed. A dry run reports changes without creating, replacing, or deleting files.

## Signals and Standard Streams

Task and Go subprocesses inherit standard input, output, and error. Interrupt and termination cancellation propagates through the lifecycle; the process runner terminates the child process tree using platform-specific behavior. Generated Web applications also translate process signals into Boot shutdown and close with a bounded context.
