# goark.build V1 规范

## 1. 目标与边界

`goark.build` 是 Goark 项目的唯一构建描述文件。Goark CLI 使用它统一描述项目入口、固定生命周期、外部工具、任务图、Profile、环境、缓存和执行策略。

本版本不包含 Agent、Plugin、运行期扫描、Shell 脚本解释器或官方 Go 工具链的替代实现。

成功标准：

- 相同源码、`go.mod`、`goark.build`、`goark.build.lock`、Profile 和声明环境在相同平台产生相同执行计划。
- 配置错误、路径逃逸、任务循环、输出冲突和工具漂移在启动任何子进程前失败。
- `goark go ...` 始终保持官方 Go 命令的透明代理语义。
- dry-run、`info` 和图查询不产生文件、进程、安装、缓存或锁文件写入副作用。
- Windows、Linux 上的参数、环境、标准流、信号和退出码行为一致。

## 2. 文件契约

- 固定名称：`goark.build`。
- 固定位置：与当前模块的 `go.mod` 同级。
- 编码：UTF-8 无 BOM。
- 换行：仅 LF。
- 语法：严格 TOML 1.0，注释使用 `#`。
- `version = 1` 必填且只接受整数 `1`。
- 未知字段、重复字段、未知任务类型、无效枚举和无效引用立即失败。
- 不读取或兼容 `goark.toml`。
- module、`go`、`toolchain` 只从 `go.mod` 读取。

需要项目描述文件的命令：

```text
run build test install vet list fix generate clean
tasks task graph sync tools tool doctor info
```

不读取项目描述文件的命令：

```text
go new codegen version help completion
```

## 3. 顶层模型

```toml
version = 1

[project]
name = "admin-minimal"
main = "./cmd/admin"
description = "Goark 最小管理系统"

[execution]
max-parallel = 4
fail-fast = true
default-timeout = "5m"

[generate]
patterns = ["./..."]
clean-stale = true
```

默认值：

- `execution.max-parallel`：当前进程 `GOMAXPROCS`，最小值为 `1`。
- `execution.fail-fast`：`true`。
- `execution.default-timeout`：`5m`。
- `generate.patterns`：`["./..."]`。
- `generate.clean-stale`：`true`。

`project.name` 为空时使用 module path 的最后一段。`project.main` 为空时沿用唯一 main package 发现规则；存在多个入口时失败。

## 4. 命令生命周期

命令配置字段：

```toml
[commands.build]
before = []
after = ["checksum"]
finally = []
go-args = ["-trimpath"]
application-args = []
output = "./build/admin-minimal"

[commands.build.environment]
CGO_ENABLED = "0"
```

只允许配置 `generate/run/build/test/install/vet/list/fix`。

普通增强命令执行顺序：

```text
加载并校验 goark.build
→ 选择 Profile
→ 合并参数和环境
→ 解析工具并校验锁文件
→ 构建并校验任务 DAG
→ commands.generate.before
→ Goark 内置代码生成
→ commands.generate.after
→ 当前命令 before
→ 官方 Go 命令
→ 成功后执行当前命令 after
→ 始终执行当前命令 finally
```

特殊规则：

- `generate` 只执行一次自身的 before、内置生成和 after；失败后执行 finally。
- `fix` 先执行 `go fix`，成功后执行完整生成生命周期，然后执行 after；finally 始终执行。
- `run/build/test/install/vet/list` 必须在官方 Go 命令前成功完成生成。
- after 只在前序主流程全部成功时执行。
- finally 按反向依赖顺序执行；失败不覆盖更早的主失败，但在主流程成功时决定最终失败。

## 5. 参数与环境

优先级从高到低：

```text
CLI 控制参数及 --goark-env
当前 Profile
commands.<name> 配置
进程环境
工具默认值
```

控制参数：

```text
--goark-profile=<name>
--goark-dry-run
--goark-offline
--goark-locked
--goark-env=KEY=VALUE
```

禁止 `--goark-no-generate` 和 `--goark-generate-only`。

`run` 参数保存为三个独立分区：

- Go 参数：传给 `go run`，保持原始顺序。
- 应用属性：`-Dkey=value` 以及 main package 后、`--` 前的属性参数。
- 应用参数：`--` 后全部原样传递。

`--goark-profile` 选择构建 Profile；`--goark.profiles.active` 是普通应用属性，二者不互相推导。

日志、JSON 和 dry-run 对名称匹配 `PASSWORD`、`PASSWD`、`SECRET`、`TOKEN`、`API_KEY`、`PRIVATE_KEY`、`CREDENTIAL` 的环境值输出 `******`。

## 6. Profile

```toml
[profiles.production]
go-args = ["-tags=production", "-trimpath"]
application-args = []

[profiles.production.environment]
GOARK_PROFILES_ACTIVE = "production"
```

未选择 Profile 时使用空 Profile。显式选择不存在的 Profile 必须失败。

## 7. 工具

```toml
[tools.goark-orm]
type = "go"
package = "goark.dev/orm/cmd/goark-orm"
version = "v0.1.0"
install = "auto"

[tools.sha256]
type = "system"
command = "sha256sum"
install = "manual"

[tools.local-linter]
type = "local"
path = "./tools/local-linter"
install = "manual"
```

规则：

- `go`：必须声明 package 和精确 version；安装到 Goark 隔离缓存，不写项目目录或全局 `GOBIN`。
- `system`：必须声明 command；只从 PATH 查找，永不自动安装。
- `local`：必须声明项目内 path；符号链接解析后仍必须位于项目根目录。
- `install` 只允许 `auto/manual`；`system/local` 只允许 `manual`。
- `goark-orm` 只是普通外部 Go 工具，CLI 不导入 ORM 包且没有 ORM 分支。

## 8. 锁文件

`goark sync` 原子生成 `goark.build.lock`。锁文件使用 UTF-8 无 BOM、LF 和稳定 TOML 排序，包含 schema 版本、生成平台以及每个工具的声明摘要和解析结果。

Go 工具按 GOOS/GOARCH 记录：

- module package 与精确版本。
- module zip 的 Go checksum database 摘要。
- 构建后可执行文件 SHA-256。

system/local 工具记录解析后的规范路径、文件 SHA-256、GOOS 和 GOARCH。锁文件不得包含环境变量值或密钥。

- `--locked`：缺失或与描述文件不一致立即失败，禁止更新。
- `--offline`：禁止网络访问和安装，只允许使用验证通过的现有工具。
- 自动恢复仅在项目已信任、锁文件完整、摘要匹配时执行。

## 9. 任务模型

用户可声明 `exec/go/delete/group`；`goark-generate` 只允许 CLI 在内存计划中注入。

```toml
[tasks.orm-generate]
type = "exec"
tool = "goark-orm"
args = ["generate", "orm", "./..."]
working-directory = "."
timeout = "2m"
depends-on = []
inputs = ["**/*.go", "resource/orm/**/*.xml"]
outputs = ["**/zz_goark_orm_*_gen.go"]
environment-inputs = ["GOARK_ORM_CONFIG"]
cache = true
parallel-safe = false
when = 'profile == "production" && goos != "windows"'

[tasks.orm-generate.environment]
GOARK_ORM_CONFIG = "resource/orm/config.toml"
```

任务图执行前必须检查：

- 循环依赖、自依赖、缺失任务和重复依赖。
- 非法工作目录、输入、输出和符号链接逃逸。
- 输出重叠与潜在父子路径冲突。
- 工具缺失或锁定信息不一致。
- cache 任务缺失 inputs 或 outputs。

并发规则：只有同时声明 `parallel-safe = true`、依赖已完成、输出不重叠的就绪任务才可并发，且总并发不超过 `max-parallel`。关闭和 finally 任务按反向依赖顺序等待已启动任务完成。

`when` 使用无副作用布尔表达式，仅允许：

- 变量：`profile`、`goos`、`goarch`、`env.NAME`。
- 操作符：`==`、`!=`、`&&`、`||`、`!` 和括号。
- UTF-8 双引号字符串、`true`、`false`。

禁止函数调用、文件访问、Shell 替换和隐式类型转换。

## 10. 变量替换

只允许：

```text
${project.root}
${project.name}
${project.module}
${profile}
${command.output}
${tool:<name>}
${env:<NAME>}
```

未知变量、缺失环境变量和递归替换立即失败。禁止反引号、`$()`、Shell 展开和字符串命令拼接。

## 11. 缓存与项目锁

任务缓存指纹包含：

- 规范化任务定义。
- 输入相对路径、类型、内容摘要和可执行位。
- 工具锁定信息。
- Go 版本、GOOS、GOARCH、Build Tags、Profile。
- environment-inputs 声明值的摘要。
- 上游输出摘要。

只有声明输出全部存在且摘要与缓存清单一致时命中。缓存条目先写同文件系统临时目录，再原子替换。所有缓存与输出提交操作受项目级跨进程锁保护。

默认目录：

```text
.goark/cache/tasks/
.goark/locks/project.lock
```

`.goark/` 必须加入脚手架 `.gitignore`。`clean` 只删除 `.goark/cache` 和当前描述文件声明且经过边界复验的输出。

## 12. 进程与安全

- 子进程必须以 executable 和 args 数组启动，禁止 Shell 字符串。
- 标准输入、标准输出和标准错误直接透传。
- Ctrl+C/SIGTERM 取消整个任务图并终止全部子进程树。
- Unix 使用独立进程组转发信号；Windows 使用 Job Object 管理进程树。
- 参数错误和项目/配置错误返回 `2`；启动或内部执行错误返回 `1`；已启动任务和 Go 命令尽可能返回其原始退出码。
- 输入、输出、工作目录和 local 工具默认不能逃出项目根目录；已存在路径必须在符号链接解析后复验。

## 13. CLI

```text
goark run/build/test/install/vet/list/fix/generate
goark clean
goark tasks [--json]
goark task <name>
goark graph [--format=text|json|dot]
goark sync [--locked] [--offline]
goark tools
goark tool install <name>
goark tool verify
goark doctor
goark info [--json]
goark go ...
goark new/codegen/version/help/completion
```

`info` 为纯只读查询，稳定 JSON 至少包含项目、Profile、工具状态、任务、生成器、缓存状态和最终执行计划。计划环境只输出命令、Profile 和 CLI 显式覆盖项，不泄露未声明的进程环境；密钥值始终脱敏。

## 14. 生成文件所有权

- 每次内置生成都重新生成完整内容，并通过同目录临时文件原子覆盖目标文件。
- 已存在目标必须具有 `// Code generated by goark; DO NOT EDIT.` 标准生成头，否则拒绝覆盖。
- 不再以“内容相同”为由保留旧文件或旧修改时间。
- `clean-stale = true` 时只删除带标准生成头且已不再对应任何有效注解 package 的文件。
- dry-run 只报告计划，不创建、覆盖或删除任何文件。

## 15. 验收

每个实现切片必须通过相关单元测试和当前仓库全量测试后独立提交。最终必须在 Windows、171dev、172dev 执行：

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go list ./...
go run ./cmd/goark info --json
go run ./cmd/goark generate --goark-dry-run
go run ./cmd/goark build --goark-dry-run
```

集成验收还必须覆盖严格解析、循环依赖、路径与符号链接逃逸、输出冲突、参数分区、环境优先级、缓存命中与失效、并发上限、取消、信号、退出码、工具安装和锁文件漂移。
