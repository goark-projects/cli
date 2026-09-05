# Goark CLI

<p align="center">
  <img src="assets/goark-readme-logo.png" alt="Goark" width="520">
</p>

<p align="center">
  基于官方 Go 工具链，为 Goark 应用提供确定性的项目编排。
</p>

<p align="center">
  <a href="README.md">English</a> | 简体中文
</p>

`goark` 是 Goark 生态的命令行入口。项目只使用一个严格 TOML 文件 `goark.build` 描述。CLI 在不替代 Go 工具链的前提下，统一编排 Goark 自有代码生成、生命周期任务、外部工具锁定、任务缓存和 Profile，并最终调用官方 Go 命令。

## 安装

需要 Go 1.25 或更高版本。

```bash
go install goark.dev/cli/cmd/goark@latest
goark version
```

## 创建项目

在当前目录创建非 Web Boot 应用：

```bash
mkdir worker && cd worker
goark new worker
go mod tidy
goark run
```

在指定目录创建 Web 应用：

```bash
goark new -type web -module example.com/admin -dir admin admin
cd admin
go mod tidy
goark run
```

`-type` 默认为 `app`，`-module` 默认使用项目名，`-dir` 默认为当前目录。两种模板都包含 Goark Boot、配置、依赖注入和 `goark.dev/gbc-log`。`web` 模板额外包含 Arkarta、Arkhos、HTTP 自动配置、健康检查端点和静态资源。

## 日常工作流

```bash
# 只读查看项目和最终执行计划。
goark info
goark info --json

# 解析工具并创建或更新 goark.build.lock。
goark sync

# 先生成 Goark 代码，再调用官方 Go 命令。
goark run
goark build
goark test ./...

# 不启动进程、不写文件，预览生命周期。
goark build --goark-dry-run

# 绕过 Goark 编排，直接执行官方 Go 命令。
goark go generate ./...
goark go test ./...
```

所有项目感知命令都要求 `goark.build` 与 `go.mod` 同级。模块路径、Go 语言版本和 toolchain 只由 `go.mod` 管理。

## 最小 `goark.build`

```toml
version = 1

[project]
name = "acb"
main = "./cmd/server"

[commands.build]
go-args = ["-trimpath"]
output = "./build/acb"

[commands.test]
go-args = ["./..."]
```

除 `version` 外，其他部分都可省略并采用安全默认值。未知字段、重复字段、非法路径、任务循环、工具缺失和锁文件漂移都会在主命令启动前失败。

## 文档

- [文档索引](docs/README.zh-CN.md)
- [快速入门](docs/getting-started.zh-CN.md)
- [`goark.build` 参考](docs/goark-build.zh-CN.md)
- [CLI 命令参考](docs/cli-reference.zh-CN.md)
- [代码生成](docs/code-generation.zh-CN.md)
- [生命周期与任务图](docs/lifecycle-and-tasks.zh-CN.md)
- [工具、锁文件、信任与缓存](docs/tools-lock-cache.zh-CN.md)
- [项目创建指南](docs/guides/project-creation.zh-CN.md)
- [CI 与离线工作流](docs/guides/ci-workflows.zh-CN.md)
- [故障排查](docs/troubleshooting.zh-CN.md)
- [版本与发布](docs/versioning-and-releases.zh-CN.md)
- [变更日志](CHANGELOG.zh-CN.md)

## 设计边界

- `goark.build` 是唯一的 Goark 项目描述文件。
- `goark go ...` 透明执行官方 Go 命令。
- `goark generate` 只运行 Goark 编译期生成器，不执行 `go generate`。
- 增强的 `run`、`build`、`test`、`install`、`vet`、`list` 都会先生成代码。
- `fix` 先执行 `go fix`，成功后重新生成。
- 外部命令始终使用“可执行文件 + 参数数组”，绝不使用 Shell 命令字符串。
- 生成的项目文件统一使用 UTF-8 无 BOM 和 LF。
- Agent 和 Plugin 不属于 `goark.build` V1 契约。

## 开发验证

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go run ./cmd/goark --help
```

## 许可证

Apache License 2.0，见 [LICENSE](LICENSE)。
