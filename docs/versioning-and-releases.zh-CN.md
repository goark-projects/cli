# 版本与发布

[English](versioning-and-releases.md) | 简体中文

Goark CLI 使用语义化版本。CLI 发布版本、项目描述文件版本和锁文件版本是三个独立契约，不能视为同一个数字。

## 版本契约

| 契约 | 示例 | 含义 |
| --- | --- | --- |
| CLI 发布版本 | `v0.0.1` | `goark` 可执行文件和 Go 模块的版本。 |
| 构建描述文件 | `version = 1` | `goark.build` 的结构版本。 |
| 锁文件 | `version = 1` | `goark.build.lock` 的结构版本。 |

CLI 版本变化不会自动改变两个文件格式。只有解析契约发生不兼容变化时，才会升级对应的文件版本。

## 1.0 之前的兼容性

- `v0.0.2` 等补丁版本用于修复缺陷，不主动改变已支持的配置和命令行为。
- `v0.1.0` 等次版本可能调整不稳定 API 或 CLI 行为，并在变更日志中提供迁移说明。
- 除非发行说明明确承诺，否则不会通过兼容垫片保留已经删除的行为。
- `goark go ...` 始终用于执行未经 Goark 扩展的官方 Go 行为。

## 版本解析

可执行文件按以下优先级确定显示版本：

1. GoReleaser 注入的发布构建版本。
2. `go install goark.dev/cli/cmd/goark@<version>` 写入的模块版本。
3. 未打标签的本地构建显示 `devel`。

命令输出不包含开头的 `v`，因此标签 `v0.0.1` 输出 `goark 0.0.1`。

## 支持的发布目标

每次发布生成以下归档：

| 操作系统 | 架构 | 格式 |
| --- | --- | --- |
| Linux | amd64、arm64 | `.tar.gz` |
| Windows | amd64、arm64 | `.zip` |
| macOS | amd64、arm64 | `.tar.gz` |

`checksums.txt` 为每个归档提供 SHA-256 摘要。V0.0.1 归档有摘要保护，但没有进行平台代码签名。

## 发布流水线

1. 在 `dev` 完成功能、测试、文档和变更日志。
2. 执行 `GOWORK=off` 测试、竞态检测、vet、工作流校验和 GoReleaser 快照。
3. 推送 `dev`，等待 Windows、Ubuntu、macOS 和 race 任务全部通过。
4. 将 `main` 快进到已验证的 `dev` 提交。
5. 从完全相同的提交创建并推送带注释的语义化版本标签。
6. 标签工作流再次进行跨平台验证，然后发布归档和校验和。
7. 校验 GitHub Release、下载归档、校验和、二进制版本和干净环境下的 `go install`。

发布工作流在校验阶段只有仓库只读权限，只有最终发布任务拥有 `contents: write`。

## 安装、升级与回滚

可复现环境应安装精确版本：

```bash
go install goark.dev/cli/cmd/goark@v0.0.1
goark version
```

使用更新标签执行同一命令即可升级。需要回滚时，重新安装指定的旧标签。`go install` 不会静默改写项目缓存和锁文件。

## 发布记录

- [变更日志](../CHANGELOG.zh-CN.md)
- [V0.0.1 发行说明](releases/v0.0.1.zh-CN.md)
