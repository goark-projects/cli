# Goark CLI 文档

[English](README.md) | 简体中文

本套文档描述当前 Goark CLI 已实现的行为。无语言后缀文件是默认英文文档，每一页都有对应的简体中文 `.zh-CN.md` 镜像。

## 学习指南

1. [快速入门](getting-started.zh-CN.md)：安装、项目结构、首次运行和标准开发循环。
2. [项目创建](guides/project-creation.zh-CN.md)：`app`、`web` 骨架和 `goark new` 的全部参数。
3. [CI 与离线工作流](guides/ci-workflows.zh-CN.md)：可复现的自动化用法。
4. [版本与发布](versioning-and-releases.zh-CN.md)：兼容性、产物、校验和与发布流水线。

## 参考手册

- [`goark.build`](goark-build.zh-CN.md)：全部配置节、字段、默认值、校验规则、变量替换和完整案例。
- [CLI 命令](cli-reference.zh-CN.md)：全部命令、参数、副作用、退出行为和使用示例。
- [代码生成](code-generation.zh-CN.md)：项目级生成、低层生成器、所有权和覆盖行为。
- [生命周期与任务](lifecycle-and-tasks.zh-CN.md)：生成顺序、任务类型、DAG 校验、并发、条件和关闭流程。
- [工具、锁文件、信任与缓存](tools-lock-cache.zh-CN.md)：工具来源、同步、验证、恢复、缓存指纹和清理。
- [故障排查](troubleshooting.zh-CN.md)：常见错误和确定性恢复步骤。
- [V0.0.1 发行说明](releases/v0.0.1.zh-CN.md)：安装、核心能力、发布产物、兼容性和边界。

## 架构决策

- [ADR-0001：固定生成与命令生命周期](adr/0001-goark-run-generation-pipeline.zh-CN.md)

## 契约摘要

| 关注点 | 契约 |
| --- | --- |
| 项目文件 | 与 `go.mod` 同级的 `goark.build` |
| 语法 | 严格 TOML 1.0、UTF-8 无 BOM、仅 LF |
| 必填字段 | `version = 1` |
| Go 元数据 | 只从 `go.mod` 读取 |
| 内置生成 | `goark generate` |
| 官方 Go 行为 | `goark go ...` |
| 工具锁文件 | `goark.build.lock` |
| 项目缓存 | `.goark/cache/tasks` |
| 构建 Profile | `--goark-profile=<name>` |
| 只读诊断 | `goark info`、`goark info --json` |

V1 项目模型明确不包含 Agent、Plugin、Shell 求值和类似 Java classpath 的运行期扫描。
