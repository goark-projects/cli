# ADR-0001: Fixed Generation and Command Lifecycle

English | [简体中文](0001-goark-run-generation-pipeline.zh-CN.md)

## Status

Accepted.

## Context

Goark uses deterministic compile-time registration instead of Java-style runtime scanning. Requiring users to run individual generators before `go run`, `go build`, or `go test` allows stale generated source and creates different local and CI behavior.

The CLI must preserve official Go toolchain semantics while adding project-owned generation, lifecycle hooks, locked external tools, Profiles, and Spring Boot-style runtime argument forwarding.

## Decision

- `goark.build` is the only project orchestration file; `go.mod` remains the only source for module path, Go version, and toolchain.
- Enhanced commands own a fixed lifecycle of validation, tool verification, generation, tasks, and an official Go subprocess.
- `run`, `build`, `test`, `install`, `vet`, and `list` generate before the Go command. `fix` executes `go fix` before regeneration.
- The lifecycle cannot skip generation. Users select raw behavior explicitly with `goark go ...`.
- `goark generate` runs only Goark-owned compile-time generators and never implicitly executes `go generate` or arbitrary directives.
- `goark codegen` remains a low-level explicit generator surface.
- A dedicated argument classifier preserves Go flags, the main target, Boot properties, application arguments, and Goark controls as separate categories.
- External tools are declared, locked, verified, and executed as an executable plus an argument array without a shell.
- Generated targets are rebuilt completely and atomically replaced only when the Goark ownership header is present.
- A cross-process project lock covers the complete lifecycle so hooks, generation, the Go subprocess, cache publication, and finalization observe one consistent project state.

## Consequences

### Positive

- One `goark run` or `goark test` command cannot accidentally consume stale Goark-generated source.
- Go compilation, module, workspace, and build cache behavior remain owned by the official Go executable.
- Configuration, graph, path, tool, and lock failures occur before the main Go command starts.
- Raw Go behavior remains clear and available rather than being approximated by bypass flags.
- The same declared lifecycle can be inspected with dry-run and `info`.

### Negative

- Enhanced commands incur project discovery, validation, and source scanning before the Go subprocess.
- The complete lifecycle lock serializes concurrent Goark commands in the same project.
- The run argument classifier must track Go build flags that consume a following value.
- Multi-main projects require `project.main` or an explicit package.

### Neutral

- Standard `go generate` remains available through `goark go generate` but is never implicit.
- A remote `package@version` has no local project-generation context and remains an official Go concern.
- Build Profiles and Boot runtime Profiles are intentionally separate controls.

## Alternatives Rejected

- **Runtime reflection scanning:** moves failures to runtime, increases startup work, and conflicts with explicit Go registration.
- **Implicit `go generate ./...`:** may execute arbitrary third-party commands outside Goark's declared tool and lock boundaries.
- **Generation bypass flags:** produce multiple meanings for the same enhanced command and weaken the fixed lifecycle.
- **Parsing all arguments with a generic CLI framework:** risks rejecting or rewriting current and future Go flags and application arguments.
- **Implementing a compiler or linker:** duplicates the Go toolchain and creates an unsustainable compatibility boundary.

## Scope

This decision covers the CLI and `goark.build` V1. Agents, Plugins, a shell interpreter, and runtime scanning are outside this scope.
