# Application Creation

English | [简体中文](project-creation.zh-CN.md)

## Command

```text
goark new [-type app|web] [-module <module-path>] [-dir <path>] <name>
```

`<name>` is the single positional argument and must appear after the flags.

| Option | Default | Purpose |
| --- | --- | --- |
| `-type app\|web` | `app` | Select a non-Web Boot application or a Web service. |
| `-module <path>` | Project name | Set the `module` directive written to `go.mod`. |
| `-dir <path>` | Current directory | Select the output directory. The CLI does not append the project name automatically. |
| `-force` | `false` | Replace files owned by the scaffold when targets already exist. |

The former `goark new app --web` command is intentionally unsupported.

## `app` Scenario

Use `app` for workers, scheduled jobs, consumers, command applications, and services that need Boot configuration and dependency injection without an HTTP server.

```bash
mkdir billing-worker
cd billing-worker
goark new -module example.com/platform/billing-worker billing-worker
```

The generated project includes:

- `//goark:application` and `//goark:configuration` application declaration.
- An annotated service interface and implementation with generated injection metadata.
- `goark.dev/gbc-log` auto-configuration.
- A minimal `main.go`; generated code owns Boot startup and shutdown.
- A minimal `goark.build` targeting `./cmd/app`.

## `web` Scenario

Use `web` for HTTP APIs, MVC applications, admin services, and static resource hosting.

```bash
goark new -type web \
  -module example.com/platform/admin \
  -dir admin \
  admin
```

The Web template adds:

- Arkarta and Arkhos dependencies.
- `goark.dev/gbc-arkhos` and `goark.dev/gbc-web` auto-configuration.
- `GET /healthz`, returning `{"status":"UP"}`.
- Annotation-generated controller, route, service injection, and application lifecycle.
- Static resources under `resource/static`.
- Graceful shutdown driven by interrupt or termination signals.
- A `goark.build` target of `./cmd/server`.

## Existing Directories

Without `-force`, generation performs a preflight check and fails before writing anything when any target file already exists. With `-force`, the scaffold writes the complete current template over those targets. Unrelated files are not removed.

Use `-force` only when the directory is version controlled or otherwise recoverable:

```bash
goark new -type web -module example.com/admin -dir . -force admin
```

## After Generation

```bash
goark generate
go mod tidy
goark info
goark run
```

The generated `resource/app.yml` contains only `goark.application.name`. Logging, the default
Arkhos address, and outbound Web client settings keep their framework defaults until an
application explicitly overrides them. `goark.web.client.*` configures outbound HTTP clients;
it is unrelated to the embedded server. Edit `goark.build` for build-time orchestration.
