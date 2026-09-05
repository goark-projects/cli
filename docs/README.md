# Goark CLI Documentation

English | [简体中文](README.zh-CN.md)

This documentation describes the behavior implemented by the current Goark CLI. Unsuffixed files are the canonical English documents; each has a Simplified Chinese `.zh-CN.md` counterpart.

## Learn

1. [Getting started](getting-started.md) explains installation, project layout, first run, and the standard development loop.
2. [Application creation](guides/project-creation.md) covers the `app` and `web` scaffolds and every `goark new` option.
3. [CI and offline workflows](guides/ci-workflows.md) provides reproducible automation patterns.

## Reference

- [`goark.build`](goark-build.md): every section, field, default, validation rule, substitution, and complete examples.
- [CLI commands](cli-reference.md): every command, arguments, side effects, exit behavior, and usage examples.
- [Code generation](code-generation.md): project generation, low-level generators, ownership, and overwrite behavior.
- [Lifecycle and tasks](lifecycle-and-tasks.md): generation ordering, task types, DAG validation, concurrency, conditions, and shutdown.
- [Tools, lock file, trust, and cache](tools-lock-cache.md): tool sources, synchronization, verification, restoration, fingerprints, and cleanup.
- [Troubleshooting](troubleshooting.md): common failures and deterministic recovery steps.

## Architecture Decisions

- [ADR-0001: Fixed generation and command lifecycle](adr/0001-goark-run-generation-pipeline.md)

## Contract Summary

| Concern | Contract |
| --- | --- |
| Project file | `goark.build`, next to `go.mod` |
| Syntax | Strict TOML 1.0, UTF-8 without BOM, LF only |
| Required field | `version = 1` |
| Go metadata | Read only from `go.mod` |
| Native generation | `goark generate` |
| Official Go behavior | `goark go ...` |
| Tool lock | `goark.build.lock` |
| Project cache | `.goark/cache/tasks` |
| Build Profile | `--goark-profile=<name>` |
| Read-only diagnostics | `goark info`, `goark info --json` |

The V1 project model intentionally excludes Agents, Plugins, shell evaluation, and runtime classpath-style scanning.
