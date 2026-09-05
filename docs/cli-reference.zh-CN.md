# CLI 命令参考

[English](cli-reference.md) | 简体中文

## 语法

```text
goark <command> [arguments]
```

使用 `goark help`、`goark help <command>`，或者命令支持时使用 `<command> --help`。诊断信息写入标准错误，结构化结果和查询结果写入标准输出。

## 命令矩阵

| 命令 | 读取 `goark.build` | 写项目文件 | 启动进程 | 作用 |
| --- | --- | --- | --- | --- |
| `run` | 是 | 生成/缓存 | 是 | 生成并运行应用。 |
| `build` | 是 | 生成/缓存/构建输出 | 是 | 生成并执行 `go build`。 |
| `test` | 是 | 生成/缓存 | 是 | 生成并执行 `go test`。 |
| `install` | 是 | 生成/缓存 | 是 | 生成并执行 `go install`。 |
| `vet` | 是 | 生成/缓存 | 是 | 生成并执行 `go vet`。 |
| `list` | 是 | 生成/缓存 | 是 | 生成并执行 `go list`。 |
| `fix` | 是 | 源码/生成/缓存 | 是 | 执行 `go fix` 后重新生成。 |
| `generate` | 是 | 生成源码/缓存 | 仅工具任务 | 运行 Goark 自有生成。 |
| `clean` | 是 | 删除声明输出/缓存 | 否 | 清理声明输出和任务缓存。 |
| `tasks` | 是 | 否 | 仅元数据发现 | 列出声明任务。 |
| `task` | 是 | 取决于任务 | 取决于任务 | 执行命名任务和依赖。 |
| `graph` | 是 | 否 | 仅元数据发现 | 输出校验后的任务图。 |
| `sync` | 是 | 锁/信任/工具缓存 | 可能 | 解析工具并更新或验证锁。 |
| `tools` | 是 | 否 | 不安装 | 报告工具状态。 |
| `tool` | 是 | 锁/信任/工具缓存 | 可能 | 安装单个工具或验证全部工具。 |
| `doctor` | 是 | 否 | Go 版本探测 | 诊断项目、任务图、Go 和工具。 |
| `info` | 是 | 否 | 仅元数据发现 | 输出稳定的只读诊断和计划。 |
| `go` | 否 | 取决于 Go 命令 | 是 | 原样执行官方 Go。 |
| `new` | 否 | 是 | 否 | 创建 `app` 或 `web` 项目。 |
| `codegen` | 否 | 可选输出 | 无子进程 | 执行低层源码生成器。 |
| `completion` | 否 | 否 | 否 | 输出 Shell 补全脚本。 |
| `help` / `version` | 否 | 否 | 否 | 输出帮助或版本。 |

dry-run 会把增强生命周期命令和 `clean` 的写入/进程行为改为只读报告。

## 通用 Goark 控制参数

增强命令接受：

| 参数 | 作用 |
| --- | --- |
| `--goark-profile=<name>` | 选择 `goark.build` 声明的 Profile。 |
| `--goark-dry-run` | 输出计划的生成、任务和 Go 命令，不写入、不启动进程。 |
| `--goark-offline` | 禁止网络访问和 Go 工具自动恢复。 |
| `--goark-locked` | 要求当前平台锁定项完整精确，并拒绝漂移。 |
| `--goark-env=KEY=VALUE` | 添加最高优先级环境覆盖，可重复。 |

已删除的 `--goark-no-generate` 和 `--goark-generate-only` 会失败。需要原始 Go 行为时使用 `goark go ...`。

## `run`

```text
goark run [go-build-flags] [package-or-go-files] [properties] [-- application-arguments]
```

```bash
goark run
goark run -race -tags=dev ./cmd/server
goark run ./cmd/server -Dserver.port=9090 --feature.enabled=true
goark run ./cmd/server -- --job=sync input.json
```

省略目标时使用 `project.main`，或者自动发现唯一 main package。分隔符前的 `-Dkey=value` 和 `--key=value` 作为 Boot 属性传入，`--` 后全部作为普通应用参数。

## 增强 Go 命令

```text
goark build [go-build-arguments]
goark test [go-test-arguments]
goark install [go-install-arguments]
goark vet [go-vet-arguments]
goark list [go-list-arguments]
goark fix [go-fix-arguments]
```

普通参数会在命令配置和 Profile 参数之后传给对应的官方 Go 子命令。Go 全局参数 `-C` 会按官方语法移到子命令前。

```bash
goark build
goark build -o ./build/custom ./cmd/server
goark test -race -count=1 ./...
goark vet ./...
goark list -deps ./...
goark install ./cmd/...
goark fix ./...
goark build -C services/admin ./...
```

对于 `build`，只有 CLI 没有提供输出参数时，`commands.build.output` 才转成 `-o <path>`。未提供构建目标且配置了 `project.main` 时，使用该 main package。

## `generate`

```text
goark generate [package-patterns] [Go-loading-flags] [--goark-*]
```

```bash
goark generate
goark generate ./internal/...
goark generate -tags=integration ./...
goark generate --goark-profile=production
```

package 模式默认使用 `generate.patterns`，其默认值为 `./...`。发现过程支持 `-C`、`-tags`、`-mod`、`-modfile`、`-overlay` 等参数。该命令不执行 `go generate`。

## `clean`

```text
goark clean [--goark-dry-run]
```

删除每个命令声明的 `output`、每个任务 `outputs` 匹配项，以及 `.goark/cache` 下的项目任务缓存。所有路径都在项目边界内解析。重要工作树应先预览：

```bash
goark clean --goark-dry-run
goark clean
```

## 任务命令

### `tasks`

```text
goark tasks [--json]
```

列出任务元数据。`--json` 提供稳定的机器可读输出。

### `task`

```text
goark task <name> [--goark-profile=<name>] [--goark-dry-run]
```

执行目标和全部上游依赖。提供通用 offline、locked 和环境控制参数时也会解析。

```bash
goark task orm-generate
goark task release --goark-profile=production --goark-locked
```

### `graph`

```text
goark graph [--format=text|json|dot]
```

默认格式为 `text`。工具集成使用 JSON，Graphviz 使用 DOT：

```bash
goark graph
goark graph --format=json
goark graph --format=dot > tasks.dot
```

## 工具命令

### `sync`

```text
goark sync [--locked] [--offline]
```

- 无参数时解析全部工具，可能自动安装符合条件的 Go 工具，写入 `goark.build.lock` 并记录项目信任。
- `--locked` 只验证，不更新锁文件。
- `--offline` 禁止网络访问和安装，但可以根据本地可解析工具更新锁与信任。

### `tools`

```text
goark tools [--json]
```

不安装工具，报告每个声明工具的 `ready`、`missing`、`unlocked`、`drift` 或 `error` 状态。

### `tool`

```text
goark tool install <name>
goark tool verify
```

`install` 显式解析或安装一个声明工具，并更新当前平台锁定项。`verify` 不安装，只校验全部声明、锁定项和可执行摘要。

## 诊断

### `doctor`

```text
goark doctor
```

检查 `goark.build`、任务图、Go 工具链可用性和每个工具，任何检查失败都会返回非零。

### `info`

```text
goark info [--goark-profile=<name>] [--json]
```

`info` 是纯只读操作，报告 CLI/Go 元数据、项目身份、当前 Profile、main package、工具状态、任务、生成器、缓存统计和全部增强命令的最终计划。它不会生成源码、安装工具、更新信任或修改锁文件。

## 官方 Go 代理

```text
goark go <go-arguments>
```

所有参数、标准流、信号、环境和可用退出码原样传递，不加载 `goark.build`，不执行生成。

```bash
goark go version
goark go env GOMOD
goark go generate ./...
goark go test ./...
```

## 项目创建

```text
goark new [-type app|web] [-module <module-path>] [-dir <path>] <name>
```

详见[项目创建指南](guides/project-creation.zh-CN.md)。

## 低层代码生成

```text
goark codegen configuration --name <name> --package <package> [flags]
goark codegen registry --package <package> --configuration <type> [flags]
goark codegen annotations --dir <package-dir> [flags]
```

未提供 `--output` 时，低层生成器输出到标准输出。重复的 `--import`、`--bean`、`--configuration` 用于构造显式声明。使用 `goark help codegen <generator>` 查看完整参数语法。普通项目应优先使用 `goark generate`。

## 补全、帮助和版本

```bash
goark completion bash
goark completion zsh
goark completion fish
goark completion powershell
goark help [command]
goark version
```

当前会话安装补全：

```bash
source <(goark completion bash)
```

```powershell
goark completion powershell | Out-String | Invoke-Expression
```

## 退出码

| 退出码 | 含义 |
| --- | --- |
| `0` | 成功。 |
| `1` | 没有更具体子进程退出码的 Goark 校验/执行失败。 |
| `2` | CLI 用法、配置或项目解析错误。 |
| `130` | 没有可用子进程退出码时的取消/中断。 |
| 其他 | 可用时保留的子进程退出码。 |
