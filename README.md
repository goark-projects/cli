# goark cli

![goark](assets/goark-readme-logo.png)

`goark cli` is the command-line tooling repository for the Goark ecosystem. It is intended to provide project scaffolding and code generation for Goark applications, including support for annotation-driven dependency injection wiring, future AOP contracts, and application configuration templates.

The project is in its early public stage. The current implementation provides explicit configuration generators and a Goark annotation scanner for core DI metadata. This module intentionally does not depend on any other Goark module; module-specific tools such as Goark ORM generation live in their own repositories.

## Goals

- Provide a Go-native scaffolding tool for Goark applications.
- Generate deterministic source code for dependency injection wiring.
- Support AOP-oriented metadata and contract generation without runtime-heavy reflection.
- Keep generated code readable, explicit, and friendly to normal Go tooling.
- Avoid hiding framework behavior behind magic global state.

## Installation

```bash
go install goark.dev/cli/cmd/goark@latest
```

During local development:

```bash
go run ./cmd/goark help
go run ./cmd/goark version
go run ./cmd/goark new app --module example.com/admin --dir admin --web
go run ./cmd/goark generate configuration --name user --package generated
go run ./cmd/goark generate registry --package generated --configuration UserConfiguration
go run ./cmd/goark generate annotations --dir internal/app --output internal/app/zz_goark_app_gen.go
```

## Current Commands

| Command | Description |
| --- | --- |
| `goark help` | Show command help. |
| `goark version` | Print the CLI version. |
| `goark new app` | Create a Goark application skeleton. |
| `goark generate configuration` | Generate a `goark.Configuration` source file. |
| `goark generate registry` | Generate a function that registers multiple `goark.Configuration` values. |
| `goark generate annotations` | Scan `//goark:*` comments and generate core registration code. |

## Application Scaffolding

`goark new app --web` creates a minimal Goark Boot Web application using
`goark.dev/gbc-web`, which includes Arkhos as the default embedded web
container.

```bash
goark new app \
  --module example.com/admin \
  --dir admin \
  --web
```

Generated files include `go.mod`, `config/app.yml`, `cmd/server/main.go`, and a
minimal MVC health endpoint under `internal/app`.

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

`goark generate annotations` scans a single Go package for `//goark:*` comments and emits same-package registration code. It supports the core annotation slice: component/service/repository, configuration/bean, autowired/qualifier/value, primary/lazy/scope/depends-on/order/priority, profile, and property-source. It also supports the current Goark Web MVC slice: controller/rest-controller/mvc-controller, request-mapping, cross-origin, GET/HEAD/POST/PUT/PATCH/DELETE/OPTIONS method mappings, request-body, request-entity, multipart-body, response-body, response-status, validated, model-attribute, path-variable, request-param, request-header, cookie-value, request-attribute, session-attribute, matrix-variable, and request-part.

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
annotation family should add its own
`AnnotationDescriptor` values, an `AnnotationBinder`, and an
`AnnotationGenerator`; it should not require scanner changes or modifications to
the core DI generator.

MVC handler parameters are explicit and generated statically. Use
`//goark:request-body[input]` for a JSON request body, use
`//goark:request-entity[request]` or a `goweb.RequestEntity[T]` parameter for a
Spring-style request entity carrying body, headers, method, and URL metadata, and use
`//goark:model-attribute[criteria]` for a query/form aggregate, and use
`//goark:path-variable[id]`, `//goark:request-param[query]`,
`//goark:request-header[requestID]`, or `//goark:cookie-value[theme]` for scalar
request values. Model attributes bind into non-pointer struct value parameters.
Scalar parameter binding currently supports `string`, `int`, `int64`, `bool`,
`float64`, and `time.Time`; path variables, request parameters, headers,
cookies, and matrix variables also support the corresponding slice forms.
`defaultValue` or `required=false` can be supplied on request parameters,
headers, and cookies.
Use `//goark:validated("create")` on routes with request-body,
request-entity, multipart-body, or model-attribute parameters to generate
explicit validation group binding.
Use `//goark:response-body` on a `controller` route when a normal return value
must be written to the response body instead of applying the controller view
resolution default. `rest-controller` already defaults normal return values to
the response body; the annotation is accepted there to keep intent explicit.
`//goark:controller-advice` and `//goark:rest-controller-advice` methods can
use `//goark:exception-handler` to return `arkarta/web.Result`,
`goark.dev/goark/web.ResponseEntity[T]`, or an ordinary value. Ordinary values
use the advice default return strategy; add `//goark:response-body` on a normal
controller advice method to force response-body output and
`//goark:response-status(...)` to set the ordinary-value HTTP status.

## ORM Generation

ORM generation is provided by the standalone `goark-orm` command from the `goark.dev/orm` module. The main `goark` CLI does not wrap ORM generation because it must remain dependency-free from other Goark modules:

```bash
goark-orm generate orm --dir internal/user --output internal/user/zz_goark_orm_user_gen.go
```

## Planned Generators

| Generator | Purpose |
| --- | --- |
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

The dependency check should show no compile-time module dependency on `goark.dev/goark`, `goark.dev/boot`, `goark.dev/orm`, or any other Goark sibling module.

## Repository Layout

```text
.
├── assets/          # README and brand assets
├── cmd/goark/       # CLI executable entrypoint
├── internal/cli/    # Command dispatch and CLI boundaries
├── internal/scaffold/ # Project skeleton generation
├── go.mod           # Go module definition
├── LICENSE          # Apache License 2.0
└── README.md        # Project overview
```

## Related Repositories

- [`goark.dev/goark`](https://goark.dev/goark): core framework contracts.
- [`goark.dev/boot`](https://goark.dev/boot): application bootstrap and convention layer.
- [`goark.dev/cli`](https://goark.dev/cli): scaffolding and code generation tooling.

## License

`goark cli` is released under the Apache License 2.0. See [LICENSE](LICENSE) for details.
