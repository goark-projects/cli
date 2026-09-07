# Changelog

English | [Simplified Chinese](CHANGELOG.zh-CN.md)

All notable changes to Goark CLI are recorded in this file. The project follows [Semantic Versioning](https://semver.org/), with the compatibility rules described in the [versioning and release policy](docs/versioning-and-releases.md).

## [Unreleased]

No unreleased changes.

## [0.0.2] - 2026-09-07

### Changed

- Require Go 1.26 or later for development, installation, and generated projects.
- Isolate generated code under each source package's `gen` package and split core,
  configuration-property, Web, and MVC output into independently owned files.
- Reorganize CLI, generator, process, path, test, and routing responsibilities into bounded
  packages without changing their public command names.

### Fixed

- Preserve valid legacy generated output when replacement generation fails validation.
- Make both project generation and low-level annotation generation enforce the same split output
  layout and deterministic stale-file cleanup.
- Honor active Go build constraints when scanning annotation source files.
- Report the actual number of generated packages in project diagnostics.
- Pin newly scaffolded projects to public Goark release tags with only directly imported modules.
- Handle canonical project paths consistently in local and hosted Windows, Linux, and macOS runs.

### Quality

- Enforce UTF-8 without BOM, LF endings, a 360-line source-file limit, a 100-character line limit,
  and at most 20 directly owned Go files per package.
- Expand compile-based generator regression coverage and isolate MVC routing policy tests.
- Update `golang.org/x/mod` and `golang.org/x/sys`; module verification and vulnerability scanning
  report no known dependency vulnerabilities.

## [0.0.1] - 2026-09-06

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

[Unreleased]: https://github.com/goark-projects/cli/compare/v0.0.2...HEAD
[0.0.2]: https://github.com/goark-projects/cli/releases/tag/v0.0.2
[0.0.1]: https://github.com/goark-projects/cli/releases/tag/v0.0.1
