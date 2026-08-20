# goark cli

![goark](assets/goark-readme-logo.png)

`goark cli` is the command-line tooling repository for the Goark ecosystem. It is intended to provide project scaffolding and code generation for Goark applications, including support for annotation-driven dependency injection wiring, future AOP contracts, and application configuration templates.

The project is in its early public stage. The current implementation provides explicit configuration generators plus a Goark annotation scanner for core DI metadata.

## Goals

- Provide a Go-native scaffolding tool for Goark applications.
- Generate deterministic source code for dependency injection wiring.
- Support AOP-oriented metadata and contract generation without runtime-heavy reflection.
- Keep generated code readable, explicit, and friendly to normal Go tooling.
- Avoid hiding framework behavior behind magic global state.

## Installation

```bash
go install github.com/goark-projects/cli/cmd/goark@latest
```

During local development:

```bash
go run ./cmd/goark help
go run ./cmd/goark version
go run ./cmd/goark generate configuration --name user --package generated
go run ./cmd/goark generate registry --package generated --configuration UserConfiguration
go run ./cmd/goark generate annotations --dir internal/app --output internal/app/zz_goark_app_gen.go
```

## Current Commands

| Command | Description |
| --- | --- |
| `goark help` | Show command help. |
| `goark version` | Print the CLI version. |
| `goark generate configuration` | Generate a `goark.Configuration` source file. |
| `goark generate registry` | Generate a function that registers multiple `goark.Configuration` values. |
| `goark generate annotations` | Scan `//goark:*` comments and generate core registration code. |

## Configuration Generation

`goark generate configuration` creates deterministic Go source that implements the core `goark.Configuration` contract structurally. The command is intentionally explicit in this phase: provider functions are supplied through flags, and future source scanners can produce the same internal generation spec.

```bash
goark generate configuration \
  --name user \
  --package generated \
  --type UserConfiguration \
  --order 100 \
  --output internal/generated/user_configuration.go \
  --bean "userRepository=NewUserRepository;lazy" \
  --bean "userService=NewUserService;deps=userRepository;primary"
```

Flags:

| Flag | Description |
| --- | --- |
| `--name` | Required configuration name returned by `Configuration.Name()`. |
| `--package` | Required generated Go package name. |
| `--type` | Generated configuration type name. Defaults to PascalCase(`--name`) + `Configuration`. |
| `--order` | Configuration ordering value. Defaults to `0`. |
| `--output` | Output file path. Defaults to stdout. |
| `--import` | Extra import in `path` or `alias=path` format. Repeatable. |
| `--bean` | Bean registration spec. Repeatable. |

Bean format:

```text
name=provider[;deps=a,b][;scope=prototype][;lazy][;primary]
```

The generated `Register` method calls `container.Register(...)`; provider expressions must be visible from the generated package.

## Registry Generation

`goark generate registry` creates the explicit registration entrypoint that replaces Spring classpath scanning in the current core-only phase.

```bash
goark generate registry \
  --package generated \
  --configuration UserConfiguration \
  --configuration HTTPConfiguration \
  --output internal/generated/registry.go
```

Flags:

| Flag | Description |
| --- | --- |
| `--package` | Required generated Go package name. |
| `--function` | Generated registry function name. Defaults to `RegisterConfigurations`. |
| `--output` | Output file path. Defaults to stdout. |
| `--import` | Extra import in `path` or `alias=path` format. Repeatable. |
| `--configuration` | Configuration type expression to instantiate and register. Repeatable. |

## Annotation Scanning

`goark generate annotations` scans a single Go package for `//goark:*` comments and emits same-package registration code. It supports the core annotation slice: component/service/repository, configuration/bean, autowired/qualifier/value, primary/lazy/scope/depends-on/order/priority, profile, and property-source.

```bash
goark generate annotations \
  --dir internal/app \
  --output internal/app/zz_goark_app_gen.go
```

Flags:

| Flag | Description |
| --- | --- |
| `--dir` | Go package directory to scan. Defaults to the current directory. |
| `--package` | Package name to scan when a directory contains multiple packages. |
| `--name` | Generated configuration name when no `//goark:configuration` exists. |
| `--type` | Generated configuration type when no `//goark:configuration` exists. |
| `--output` | Output file path. Defaults to stdout. |

Annotation handling is deliberately extension-based. The scanner only parses Go
syntax, validates registered descriptors, and dispatches matching items. A new
annotation family, such as future ORM/MyBatis metadata, should add its own
`AnnotationDescriptor` values, an `AnnotationBinder`, and an
`AnnotationGenerator`; it should not require scanner changes or modifications to
the core DI generator.

## Planned Generators

| Generator | Purpose |
| --- | --- |
| `goark new` | Create a Goark application skeleton. |
| `goark aop` | Generate AOP contracts and weaving metadata. |

## Repository Status

This repository is an early skeleton. Public commands and generated file formats should be treated as unstable until the first tagged release.

## Development

Requirements:

- Go 1.25 or later
- Git

Useful commands:

```bash
go fmt ./...
go list ./...
go run ./cmd/goark help
```

## Repository Layout

```text
.
├── assets/          # README and brand assets
├── cmd/goark/       # CLI executable entrypoint
├── internal/cli/    # Command dispatch and CLI boundaries
├── go.mod           # Go module definition
├── LICENSE          # Apache License 2.0
└── README.md        # Project overview
```

## Related Repositories

- [`goark-projects/goark`](https://github.com/goark-projects/goark): core framework contracts.
- [`goark-projects/boot`](https://github.com/goark-projects/boot): application bootstrap and convention layer.
- [`goark-projects/cli`](https://github.com/goark-projects/cli): scaffolding and code generation tooling.

## License

`goark cli` is released under the Apache License 2.0. See [LICENSE](LICENSE) for details.
