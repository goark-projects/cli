# goark cli

![goark](assets/goark-readme-logo.png)

`goark cli` is the command-line tooling repository for the Goark ecosystem. It is intended to provide project scaffolding and code generation for Goark applications, including future support for AOP contracts, dependency injection wiring, and application module templates.

The project is in its initial public bootstrap stage. The current implementation provides a minimal executable command boundary so the repository can compile, install, and evolve without pretending that the generators already exist.

## Goals

- Provide a Go-native scaffolding tool for Goark applications.
- Generate deterministic source code for dependency injection wiring.
- Support AOP-oriented metadata and contract generation without runtime-heavy reflection.
- Keep generated code readable, explicit, and friendly to normal Go tooling.
- Avoid hiding framework behavior behind magic global state.

## Module

```bash
go install github.com/goark-projects/cli/cmd/goark@latest
```

During local development:

```bash
go run ./cmd/goark help
go run ./cmd/goark version
```

## Current Commands

| Command | Description |
| --- | --- |
| `goark help` | Show command help. |
| `goark version` | Print the CLI version. |

## Planned Generators

| Generator | Purpose |
| --- | --- |
| `goark new` | Create a Goark application skeleton. |
| `goark aop` | Generate AOP contracts and weaving metadata. |
| `goark di` | Generate dependency injection wiring code. |

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
