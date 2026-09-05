# Changelog

English | [Simplified Chinese](CHANGELOG.zh-CN.md)

All notable changes to Goark CLI are recorded in this file. The project follows [Semantic Versioning](https://semver.org/), with the compatibility rules described in the [versioning and release policy](docs/versioning-and-releases.md).

## [Unreleased]

No unreleased changes.

## [0.0.1] - 2026-09-05

### Added

- Strict `goark.build` V1 parsing and validation.
- Fixed generation lifecycles for `run`, `build`, `test`, `install`, `vet`, `list`, `fix`, and `generate`.
- Typed task DAGs with dependency validation, bounded concurrency, conditions, finalizers, timeouts, and cancellation.
- Isolated Go, system, and local tool resolution with project trust and `goark.build.lock` verification.
- Content-verified task caching and cross-process project locking.
- Build Profiles, deterministic environment precedence, safe variable expansion, and secret redaction.
- Read-only project diagnostics, task inspection, graph output, tool management, shell completion, and the transparent `goark go` proxy.
- Compile-time DI, configuration, AOP, MVC, and Web code generation with deterministic replacement of owned generated files.
- Simplified `app` and `web` project scaffolds with `goark.dev/gbc-log` included by default.
- English-first bilingual documentation, cross-platform CI, and reproducible release archives with SHA-256 checksums.

[Unreleased]: https://github.com/goark-projects/cli/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/goark-projects/cli/releases/tag/v0.0.1
