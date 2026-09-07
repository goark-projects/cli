# 变更日志

[English](CHANGELOG.md) | 简体中文

本文件记录 Goark CLI 的重要变更。项目遵循[语义化版本](https://semver.org/lang/zh-CN/)，兼容性规则见[版本与发布策略](docs/versioning-and-releases.zh-CN.md)。

## [未发布]

暂无未发布变更。

## [0.0.3] - 2026-09-07

### 变更

- 在生成子包中完成注解式 App、Web 启动、Service 与接口注入。
- 精简应用 YAML，新生成项目最低要求 Go 1.27。
- CI 和发布构建采用 Go 1.27，以验证新生成项目。

### 修复

- 补齐新项目依赖，同时保留显式及持久化模块模式。
- 包发现遵循命令最终环境与 Go 工作区。
- 测试透传参数和参数值不再影响构建及缓存决策。
- 拒绝重复应用注解，并保留启动失败诊断。

### 质量

- 增加真实子进程参数与依赖回归测试、共享参数扫描及模糊测试。
- 验证公共模块骨架、Web 生命周期错误、静态分析及已知漏洞。

## [0.0.2] - 2026-09-07

### 变更

- 开发、安装及生成项目的最低 Go 版本调整为 Go 1.26。
- 将生成代码隔离到各源码包的 `gen` 包，并将核心注册、配置属性、Web、MVC 输出拆分为
  独立所有权文件。
- 按职责重新组织 CLI、生成器、进程、路径、测试和路由代码，保持公开命令名称不变。

### 修复

- 新生成结果校验失败时保留有效的旧版生成文件。
- 项目生成和底层注解生成统一采用拆分输出布局及确定性的旧文件清理规则。
- 扫描注解源码时遵循当前启用的 Go 构建约束。
- 项目诊断输出准确统计实际生成的包数量。
- 新建项目固定引用 Goark 生态正式标签，并仅声明源码直接导入的模块。
- 统一处理本机及托管 Windows、Linux、macOS 环境中的规范项目路径。

### 质量

- 自动强制 UTF-8 无 BOM、LF 换行、单文件不超过 360 行、单行不超过 100 字符，
  且每个包直属 Go 文件不超过 20 个。
- 扩充生成代码编译回归覆盖，并隔离 MVC 路由策略测试。
- 更新 `golang.org/x/mod` 和 `golang.org/x/sys`；模块校验和漏洞扫描未发现已知依赖漏洞。

## [0.0.1] - 2026-09-06

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

[未发布]: https://github.com/goark-projects/cli/compare/v0.0.3...HEAD
[0.0.3]: https://github.com/goark-projects/cli/releases/tag/v0.0.3
[0.0.2]: https://github.com/goark-projects/cli/releases/tag/v0.0.2
[0.0.1]: https://github.com/goark-projects/cli/releases/tag/v0.0.1
