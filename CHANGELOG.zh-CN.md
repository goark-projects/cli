# 变更日志

[English](CHANGELOG.md) | 简体中文

本文件记录 Goark CLI 的重要变更。项目遵循[语义化版本](https://semver.org/lang/zh-CN/)，兼容性规则见[版本与发布策略](docs/versioning-and-releases.zh-CN.md)。

## [未发布]

暂无未发布变更。

## [0.0.1] - 2026-09-05

### 新增

- 严格的 `goark.build` V1 解析与校验。
- `run`、`build`、`test`、`install`、`vet`、`list`、`fix`、`generate` 固定生成生命周期。
- 支持依赖校验、有界并发、条件、收尾任务、超时和取消的类型化任务 DAG。
- 隔离的 Go、系统、本地工具解析，以及项目信任和 `goark.build.lock` 校验。
- 内容校验任务缓存和跨进程项目锁。
- 构建 Profile、确定性环境优先级、安全变量替换和密钥脱敏。
- 只读项目诊断、任务查看、图输出、工具管理、Shell 补全和透明的 `goark go` 代理。
- 编译期 DI、配置、AOP、MVC、Web 代码生成，并确定性覆盖 Goark 所有的生成文件。
- 精简的 `app`、`web` 项目骨架，默认包含 `goark.dev/gbc-log`。
- 英文优先的双语文档、跨平台 CI，以及包含 SHA-256 校验和的可复现发布归档。

[未发布]: https://github.com/goark-projects/cli/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/goark-projects/cli/releases/tag/v0.0.1
