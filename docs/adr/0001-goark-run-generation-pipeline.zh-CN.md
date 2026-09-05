# ADR-0001：固定生成与命令生命周期

[English](0001-goark-run-generation-pipeline.md) | 简体中文

## 状态

已接受。

## 背景

Goark 使用确定性的编译期注册代替 Java 风格运行期扫描。如果要求用户在 `go run`、`go build`、`go test` 前手工运行单个生成器，就可能使用过期生成源码，并导致本地与 CI 行为不同。

CLI 必须保留官方 Go 工具链语义，同时加入项目自有生成、生命周期钩子、锁定外部工具、Profile 和 Spring Boot 风格运行参数传递。

## 决策

- `goark.build` 是唯一项目编排文件；`go.mod` 仍是 module path、Go 版本和 toolchain 的唯一来源。
- 增强命令拥有固定的校验、工具验证、生成、任务和官方 Go 子进程生命周期。
- `run`、`build`、`test`、`install`、`vet`、`list` 在 Go 命令前生成；`fix` 在重新生成前执行 `go fix`。
- 生命周期不能跳过生成，需要原始行为时显式使用 `goark go ...`。
- `goark generate` 只运行 Goark 自有编译期生成器，绝不隐式执行 `go generate` 或任意 directive。
- `goark codegen` 保留为低层显式生成器入口。
- 专用参数分类器将 Go flag、main 目标、Boot 属性、应用参数和 Goark 控制参数严格分区。
- 外部工具必须声明、锁定、验证，并使用“可执行文件 + 参数数组”执行，不经过 Shell。
- 生成目标每次完整重建，并且只有存在 Goark 所有权标头时才原子替换。
- 跨进程项目锁覆盖完整生命周期，使钩子、生成、Go 子进程、缓存发布和收尾过程观察同一个一致项目状态。

## 结果

### 正面

- 一次 `goark run` 或 `goark test` 不会意外使用过期 Goark 生成源码。
- Go 编译、模块、工作区和构建缓存行为仍由官方 Go 可执行文件管理。
- 配置、任务图、路径、工具和锁错误在主 Go 命令启动前失败。
- 原始 Go 行为通过清晰入口保留，不需要近似的绕过开关。
- dry-run 和 `info` 可以检查同一个声明生命周期。

### 负面

- 增强命令在 Go 子进程前增加项目发现、校验和源码扫描成本。
- 完整生命周期锁会串行化同一项目内的并发 Goark 命令。
- run 参数分类器需要持续跟踪会消费下一个值的 Go 构建参数。
- 多 main 项目需要声明 `project.main` 或显式传 package。

### 中性

- 标准 `go generate` 仍可通过 `goark go generate` 使用，但绝不隐式执行。
- 远程 `package@version` 没有本地项目生成上下文，仍属于官方 Go 行为。
- 构建 Profile 与 Boot 运行期 Profile 刻意保持为独立控制项。

## 已拒绝方案

- **运行期反射扫描：** 把失败延迟到运行期、增加启动工作量，并违背 Go 显式注册原则。
- **隐式执行 `go generate ./...`：** 可能运行声明工具和锁边界之外的任意第三方命令。
- **生成绕过参数：** 让同一个增强命令具有多个含义，破坏固定生命周期。
- **使用通用 CLI 框架解析全部参数：** 可能拒绝或重写当前及未来 Go flag 和应用参数。
- **自行实现编译器或链接器：** 重复 Go 工具链并制造不可持续的兼容边界。

## 范围

本决策覆盖 CLI 和 `goark.build` V1。Agent、Plugin、Shell 解释器和运行期扫描不在范围内。
