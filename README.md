# goark cli

![goark](assets/goark-readme-logo.png)

`goark` 是 Goark 项目的命令行入口。`goark.build` 是唯一项目描述文件，用于统一工具、任务图、生命周期、Profile、环境、缓存和锁文件。CLI 在标准 Go 工具链前执行确定性的编译期代码生成，同时通过隔离的 `goark go ...` 命名空间完整代理本机 Go 命令。

## 安装

```bash
go install goark.dev/cli/cmd/goark@latest
```

## 核心工作流

```bash
# 校验并同步项目工具锁文件
goark sync

# 自动发现 main package、生成代码并运行
goark run

# 生成后构建或测试
goark build ./...
goark test -race ./...

# 只生成 Goark 编译期代码
goark generate

# 查看当前项目、工具链、入口和生成计划
goark info
goark info --json
goark info --goark-profile=production --json

# 原样执行官方 Go 命令，不注入 Goark 行为
goark go version
goark go env GOMOD
goark go generate ./...
goark go build ./...
```

## 命令

| 命令 | 说明 |
| --- | --- |
| `goark run` | 生成代码并运行 Goark 应用，未指定入口时自动发现。 |
| `goark build` | 生成代码并执行 `go build`。 |
| `goark test` | 生成代码并执行 `go test`。 |
| `goark install` | 生成代码并执行 `go install`。 |
| `goark vet` | 生成代码并执行 `go vet`。 |
| `goark list` | 生成代码并执行 `go list`。 |
| `goark fix` | 执行 `go fix`，成功后重新生成代码。 |
| `goark generate` | 扫描项目并生成全部 Goark CLI 自有编译期代码。 |
| `goark clean` | 删除声明的项目输出和任务缓存。 |
| `goark tasks [--json]` | 列出任务。 |
| `goark task <name>` | 执行指定任务及其依赖。 |
| `goark graph` | 以 text、JSON 或 DOT 输出任务图。 |
| `goark sync` | 解析工具并更新或校验 `goark.build.lock`。 |
| `goark tools` | 查看工具状态。 |
| `goark tool install/verify` | 安装单个 Go 工具或校验全部工具。 |
| `goark doctor` | 诊断描述文件、任务图、Go 工具链和外部工具。 |
| `goark info` | 输出默认或指定 Profile 的项目与最终计划，不写文件。 |
| `goark go ...` | 原样代理官方 Go 命令。 |
| `goark codegen ...` | 执行低层显式代码生成器。 |
| `goark new [-type app|web] <name>` | 创建 Goark Boot 应用骨架。 |
| `goark version` | 输出 Goark CLI 版本。 |
| `goark completion <shell>` | 输出 Bash、Zsh、Fish 或 PowerShell 补全脚本。 |

## 运行参数

`goark run` 保留 Go 的构建参数，并把 Goark 属性放到 main package 后传给应用：

```bash
goark run -race -tags=dev ./cmd/server
goark run -Dserver.port=9090 -Dgoark.profiles.active=dev
goark run --server.port=9090 --goark.profiles.active=dev
goark run ./cmd/server -- --job=sync input.json
```

配置优先级从高到低：

1. `--key=value` 应用命令行属性。
2. `-Dkey=value` 系统属性。
3. 操作系统环境变量。
4. Profile 配置文件。
5. 基础配置文件。
6. 框架默认值。

环境变量支持松散名称映射，例如 `SERVER_PORT` 对应 `server.port`：

```bash
SERVER_PORT=9090 GOARK_PROFILES_ACTIVE=dev goark run
```

PowerShell：

```powershell
$env:SERVER_PORT = "9090"
$env:GOARK_PROFILES_ACTIVE = "dev"
goark run
```

Goark 增强命令的控制参数：

| 参数 | 说明 |
| --- | --- |
| `--goark-profile=<name>` | 选择 `goark.build` 中声明的 Profile。 |
| `--goark-dry-run` | 输出生成和 Go 命令计划，不写文件、不执行。 |
| `--goark-offline` | 禁止网络和自动恢复工具。 |
| `--goark-locked` | 要求现有锁文件完整匹配。 |
| `--goark-env=KEY=VALUE` | 覆盖一个执行环境变量，可重复。 |

固定生命周期不能跳过生成。需要完全原始的 Go 行为时使用 `goark go ...`。

`go test -args` 和 `goark run --` 之后的参数不会再被 Goark 解析。

`goark build/test/install/vet/list/fix` 接受对应 Go 子命令参数。Go 全局 `-C` 参数可直接使用，例如 `goark build -C services/admin ./...`；CLI 会按官方语法执行 `go -C services/admin build ./...`。`GOFLAGS`、`GOOS`、`GOARCH`、`GOWORK`、`GOTOOLCHAIN` 等环境变量由子进程原样继承。

## 项目描述

增强命令要求模块根目录存在 UTF-8 无 BOM、LF 换行的 `goark.build`：

```toml
version = 1

[project]
name = "admin-minimal"
main = "./cmd/admin"

[commands.build]
go-args = ["-trimpath"]
output = "./build/admin-minimal"

[commands.test]
go-args = ["-count=1", "./..."]
```

未知字段、重复字段、非法路径和无效引用直接失败。module、Go 版本和 toolchain 只读取同级 `go.mod`。完整规范见 [goark.build V1](docs/spec/goark-build-v1.md) 和 [English specification](docs/spec/goark-build-v1.en.md)。

## 项目发现

`goark run` 未指定 package 时按以下顺序解析入口：

1. 当前目录是 `package main` 时使用 `.`。
2. `./cmd/...` 下唯一的 main package。
3. 多入口时失败并列出全部候选项，要求显式指定运行目标。

## 编译期生成

项目级生成器基于 `go list` 的实际构建文件集运行，遵守 `-tags`、`-overlay`、`-mod` 和 `-modfile`。当前生成范围包括：

- Component、Service、Repository、Configuration 和 Bean 注册。
- Autowired、Inject、Qualifier、Named、Resource 和 Value 注入。
- Profile、Conditional、Lazy、Scope、DependsOn、Order 和 Priority 元数据。
- ConfigurationProperties 绑定元数据。
- Web/MVC Controller、路由、参数绑定、返回值和 Advice 注册。

生成文件名为 `zz_goark_<package>_gen.go`。每次生成都通过同目录临时文件原子覆盖具有 Goark 标准生成头的目标文件，确保旧生成结果不会残留；同名手写文件会阻止覆盖。源码移除相关注解后，CLI 只删除具有 Goark 生成文件头的对应陈旧文件。

同一项目上的并发 `run/build/test/generate` 使用跨进程项目锁串行化生成阶段，避免多个进程同时替换生成文件；锁只覆盖生成，不覆盖后续 Go 构建或应用运行。

CLI 不会隐式执行 `go generate ./...` 或任意第三方命令。需要标准 Go 生成行为时显式使用：

```bash
goark go generate ./...
```

## Shell 补全

```bash
# Bash
source <(goark completion bash)

# Zsh
source <(goark completion zsh)

# Fish
goark completion fish | source
```

PowerShell：

```powershell
goark completion powershell | Out-String | Invoke-Expression
```

## 低层代码生成

项目工作流通常只需要 `goark generate`。需要精确控制单个输出时可使用：

```bash
goark codegen annotations --dir internal/app --output internal/app/zz_goark_app_gen.go
goark codegen configuration --name user --package generated --output internal/generated/user_configuration.go
goark codegen registry --package generated --configuration UserConfiguration --output internal/generated/registry.go
```

注解生成管线由 `AnnotationDescriptor`、`AnnotationBinder` 和 `AnnotationGenerator` 组成。新增注解族不修改主扫描器或其他生成器。

## 项目骨架

```bash
goark new -type web -module example.com/admin -dir admin admin
cd admin
go mod tidy
goark run
```

`-type` 默认为 `app`。`app` 生成不包含 HTTP 服务的 Boot、配置与依赖注入骨架；`web` 额外生成 Arkarta/Arkhos Web 服务、健康检查和静态资源。两类骨架都默认集成 `goark.dev/gbc-log`。`-dir` 默认当前目录，`-module` 默认使用项目名。

## 开发验证

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go list ./...
go run ./cmd/goark --help
go run ./cmd/goark go version
```

CLI 不编译依赖任何 Goark 运行时兄弟模块。代码和生成文件统一使用 UTF-8、LF。

## License

Apache License 2.0，见 [LICENSE](LICENSE)。
