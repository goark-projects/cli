# 快速入门

[English](getting-started.md) | 简体中文

## 前置条件

- Go 1.25 或更高版本。
- 受支持的 Windows、Linux 或 macOS 环境。
- 声明的 Go 工具需要安装时，应具备 Git 和网络访问能力。

安装 CLI：

```bash
go install goark.dev/cli/cmd/goark@latest
```

确保 Go 的二进制目录位于 `PATH`，然后验证：

```bash
goark version
goark help
```

## 创建第一个应用

最短命令会在当前目录创建 `app` 项目。省略 `-module` 时，项目名同时作为 module path。

```bash
mkdir hello
cd hello
goark new hello
```

创建 Web 服务时，通常应显式提供全局唯一 module path：

```bash
goark new -type web -module example.com/team/hello -dir hello hello
cd hello
```

生成的入口刻意保持简洁：

```go
package main

import "os"

func main() {
	os.Exit(runGoark(os.Args[1:]))
}
```

同目录 `goark.go` 负责 Boot 启动、参数传递、自动配置、需要的信号处理、关闭流程和进程退出码。

## 生成目录

`app` 项目包含：

```text
.
|-- go.mod
|-- goark.build
|-- resource/app.yml
|-- cmd/app/main.go
|-- cmd/app/goark.go
`-- internal/app/configuration.go
```

`web` 项目使用 `cmd/server`，增加 `resource/static/index.html`，并注册 Arkhos 服务、MVC 路由、HTTP 客户端定制和 `GET /healthz`。

两种模板都包含 `goark.dev/gbc-log` 并注册其自动配置。

## 准备与检查

先解析模块依赖：

```bash
go mod tidy
```

`goark info` 是最适合作为第一步的项目命令。它会验证项目并报告当前 Profile、Go 元数据、main package、工具、任务、生成器扫描、缓存和最终执行计划，但不会安装工具或生成代码。

```bash
goark info
goark info --json
```

如果 `goark.build` 声明了工具，应先同步：

```bash
goark sync
goark tool verify
```

应提交 `goark.build.lock`，让 CI 和其他开发者验证完全一致的工具身份。

## 运行、构建与测试

```bash
goark run
goark build
goark test ./...
goark vet ./...
```

这些增强命令会校验 `goark.build`、选择构建 Profile、解析所需锁定工具、运行 Goark 生成、执行生命周期任务，最后调用官方 Go 命令。

无副作用预览：

```bash
goark run --goark-dry-run
goark build --goark-dry-run
```

不需要 Goark 编排时，直接使用官方 Go 行为：

```bash
goark go env GOMOD
goark go generate ./...
goark go test ./...
```

## 运行参数

`goark run` 将四类参数严格分区：

```bash
goark run -race -tags=dev ./cmd/server \
  -Dserver.port=9090 \
  --goark.profiles.active=dev \
  -- --job=sync input.json
```

| 分区 | 去向 |
| --- | --- |
| `-race -tags=dev` | Go 构建参数 |
| `./cmd/server` | main package |
| `-Dserver.port=9090` | Boot 系统属性 |
| `--goark.profiles.active=dev` | Boot 命令行属性 |
| `--` 后的内容 | 普通应用参数 |

`--goark-profile=dev` 选择的构建 Profile 与 Boot 运行时属性 `--goark.profiles.active=dev` 彼此独立。如果构建期和运行期都需要切换，应同时配置两者。

## 后续阅读

- 完整 [`goark.build` 参考](goark-build.zh-CN.md)。
- [固定生命周期与任务图](lifecycle-and-tasks.zh-CN.md)。
- [CLI 命令参考](cli-reference.zh-CN.md)。
