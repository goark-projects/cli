# Goark Go 命令兼容层 V1 规范

## 目标

`goark` 同时提供 Goark 自有命令和隔离的 Go 命令代理。`goark go ...` 后面的全部参数原样委托给本机 `go`；Goark 自有的 `run/build/test/install/vet/list/fix` 在执行对应 Go 命令前自动生成编译期代码。

成功标准：

- 在标准 Goark 项目根目录执行 `goark run`，无需指定 main package。
- `goark build/test/install/vet/list/fix/run` 兼容对应的 Go 命令参数并执行前置生成。
- `goark go <arguments>` 透明代理全部官方 Go 命令。
- 支持 `-Dkey=value` 系统属性、`--key=value` 命令行属性和操作系统环境变量。
- 编译前自动生成 CLI 自身拥有的注解、依赖注入、配置绑定和 Web/MVC 注册代码。
- 生成失败时不执行 `go run`，并返回非零退出码。
- 保持子进程标准输入、标准输出、标准错误、环境变量和中断信号语义。

## 命令契约

```text
goark go [go-global-flags] <go-command> [arguments]
goark run [go-build-flags] [package-or-go-files] [goark-properties] [-- application-arguments]
goark generate [package-patterns]
goark codegen <generator> [arguments]
goark info [--goark-profile=<name>] [--json]
goark completion <bash|zsh|fish|powershell>
```

示例：

```bash
goark run
goark run -race -tags=dev ./cmd/server
goark run -Dserver.port=9090 -Dgoark.profiles.active=dev
goark run --server.port=9090 --goark.profiles.active=dev
goark run ./cmd/server -- --job=sync input.json
goark build -trimpath ./...
goark test -race ./...
goark vet ./...
goark generate
goark go -C services/admin build ./...
goark go generate ./...
goark go version
```

`generate` 是新的项目级自动生成命令。`codegen` 是 Goark 编译期生成器的低层显式接口。标准 `go generate` 只能通过 `goark go generate` 使用，不提供旧入口或兼容分支。

生成触发矩阵：

- 前置生成：Goark 自有的 `run`、`build`、`test`、`install`、`vet`、`list`、`fix`。
- 透明代理：`goark go` 后面的全部参数。
- `goark generate` 只执行 Goark 自有生成器，不隐式执行任意 `go:generate` 指令。

参数分层：

- `-Dkey=value` 是 Goark 系统属性，不传给 `go` 命令的构建参数解析器。
- main package 前的 `--key=value` 是 Goark 应用命令行属性。
- `run` 命令中 `--` 之后的全部参数原样传给应用。
- 其他 Go 构建参数、包参数和 Go 文件参数保持原有顺序传给 `go run`。
- 需要与 Go 构建参数同名的应用参数时，必须放在 `--` 之后。

CLI 控制参数使用 `--goark-` 前缀：

- `--goark-profile=<name>`：选择 `goark.build` Profile。
- `--goark-dry-run`：显示执行计划，不写文件、不安装工具、不启动进程。
- `--goark-offline`：禁止网络访问和自动工具恢复。
- `--goark-locked`：要求锁文件完整匹配。
- `--goark-env=KEY=VALUE`：覆盖执行环境变量，可重复。

固定生命周期不能跳过生成；需要原始 Go 行为时使用 `goark go ...`。

## 项目发现

项目根目录由包含 `goark.build` 的本地 Go 模块确定。未显式指定 main package 时优先使用 `[project].main`，未配置时按以下顺序选择：

1. 当前目录为 `package main` 时使用 `.`。
2. `./cmd/...` 下只有一个 main package 时使用该包。
3. 多个候选入口时失败并列出候选项。

远程 `package@version` 不属于本地 Goark 项目，跳过生成并直接委托对应 Go 命令。

## 生成阶段

默认扫描当前模块的 `./...` package。只有包含 `//goark:` 注解的 package 才进入生成器。

V1 生成范围：

- Component、Service、Repository、Configuration 和 Bean 注册。
- 依赖注入、Qualifier、Value、Profile、Conditional 和生命周期元数据。
- ConfigurationProperties 绑定元数据。
- Web/MVC Controller、路由、参数绑定、返回值和 Advice 注册。

生成文件使用 `zz_goark_<package>_gen.go`，必须 UTF-8、LF、确定性排序。每次生成都原子覆盖带 Goark 标准生成头的目标文件；同名手写文件拒绝覆盖。生成器只处理自身协议，不隐式执行 `go generate ./...` 或任意第三方命令。

## 配置优先级

从高到低：

1. `--key=value` 应用命令行属性。
2. `-Dkey=value` 系统属性。
3. 操作系统环境变量。
4. Profile 配置文件。
5. 基础配置文件。
6. 框架默认值。

同一层级重复属性以后出现者为准。

## 退出与失败契约

- 参数错误、项目发现错误、配置错误：退出码 `2`。
- 生成或进程启动失败：退出码 `1`。
- `go run` 已启动后，返回 `go` 进程自身退出码。
- 所有透明代理命令返回 `go` 进程自身退出码。
- 诊断信息写入 stderr；应用 stdout 不被 CLI 包装。
- 收到中断信号时不吞掉信号，终端控制权由父子进程共享；Unix 信号退出映射为 `128+signal`。

## 并发与工作区

- 多模块 `go.work` 中选择包含当前工作目录且路径最深的本地模块。
- `-C` 按 Go 全局参数处理，项目发现目录与最终 Go 命令保持一致。
- `GOFLAGS`、`GOOS`、`GOARCH`、`GOWORK`、`GOTOOLCHAIN` 原样继承。
- 同一项目的生成阶段使用跨进程文件锁串行化，锁不覆盖后续 Go 命令。
- `goark info --json` 输出稳定字段，供脚本和 IDE 集成使用。

## 验证

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go list ./...
go run ./cmd/goark run --goark-dry-run
go run ./cmd/goark build --goark-dry-run ./...
go run ./cmd/goark go env GOMOD
```

还需在 Windows、Debian 171dev 和 Debian 172dev 验证参数透传、生成幂等、应用启动、Ctrl+C/SIGTERM 和错误退出。
