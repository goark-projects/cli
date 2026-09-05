# `goark.build` 参考

[English](goark-build.md) | 简体中文

`goark.build` 是唯一的 Goark 项目描述文件。它管理构建期编排，包括项目入口、代码生成、命令钩子、外部工具、任务、Profile、环境、并发和缓存。它不会替代 `go.mod` 或应用运行期配置。

## 文件契约

| 属性 | 要求 |
| --- | --- |
| 文件名 | 固定为 `goark.build` |
| 位置 | 与 `go.mod` 同级 |
| 语法 | TOML 1.0，使用 `#` 注释 |
| 编码 | 有效 UTF-8，无 BOM |
| 换行 | 只能使用 LF，拒绝 CRLF 和 CR |
| 必填字段 | 整数 `version = 1` |
| 解析规则 | 未知字段和重复字段直接失败 |

module path、Go 语言版本和 `toolchain` 只从 `go.mod` 读取，不能在这里重复配置。

## 最小配置

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

只有 `version` 必填。在项目真正需要钩子、外部工具、缓存或 Profile 前，推荐保持这种精简形式。

## 顶层配置

| 配置节 | 必填 | 作用 |
| --- | --- | --- |
| `version` | 是 | 选择文件格式，目前只支持整数 `1`。 |
| `[project]` | 否 | 项目人类可读信息和默认 main package。 |
| `[execution]` | 否 | 全局任务调度与超时策略。 |
| `[generate]` | 否 | Goark 自有编译期生成范围。 |
| `[commands.<name>]` | 否 | 固定命令的钩子、参数、环境和输出。 |
| `[tools.<name>]` | 否 | 外部可执行工具声明。 |
| `[tasks.<name>]` | 否 | 任务图节点。 |
| `[profiles.<name>]` | 否 | 命名的构建期参数与环境覆盖层。 |

## `[project]`

```toml
[project]
name = "admin"
main = "./cmd/server"
description = "管理服务"
```

| 字段 | 类型 | 默认值 | 作用 |
| --- | --- | --- | --- |
| `name` | 字符串 | `go.mod` module path 最后一段 | 显示名称和 `${project.name}` 的值。 |
| `main` | 项目相对路径字符串 | 自动发现 | `run` 的默认 main package；`build` 未指定目标时也使用它。 |
| `description` | 字符串 | 空 | 由 `info` 报告的人类可读元数据。 |

入口发现会优先使用当前目录的 `main` package，否则查找 `./cmd/...` 下唯一 main package。存在多个候选时必须显式指定目标。

## `[execution]`

```toml
[execution]
max-parallel = 4
fail-fast = true
default-timeout = "5m"
```

| 字段 | 类型 | 默认值 | 作用 |
| --- | --- | --- | --- |
| `max-parallel` | 正整数 | 当前 `GOMAXPROCS`，最小为 1 | 同时执行的就绪任务节点上限。 |
| `fail-fast` | 布尔值 | `true` | 首个任务失败后取消剩余可调度工作。 |
| `default-timeout` | Go duration 字符串 | `5m` | 未声明 `timeout` 的任务使用；同时限制 `finally` 处理。 |

duration 使用 Go 语法，例如 `500ms`、`30s`、`2m`、`1h30m`，这里必须为正值。

## `[generate]`

```toml
[generate]
patterns = ["./..."]
clean-stale = true
```

| 字段 | 类型 | 默认值 | 作用 |
| --- | --- | --- | --- |
| `patterns` | 字符串数组 | `["./..."]` | 内置生成器扫描的项目内 Go package 模式。 |
| `clean-stale` | 布尔值 | `true` | 只删除带 Goark 生成文件所有权标头的过期文件。 |

模式必须是 `.` 或以 `./` 开头，并且不能逃出项目。生成会遵守 `-tags`、`-mod`、`-modfile`、`-overlay` 等相关 Go 加载参数。

## `[commands.<name>]`

支持 `generate`、`run`、`build`、`test`、`install`、`vet`、`list`、`fix`。

```toml
[commands.build]
before = ["assets"]
after = ["checksum"]
finally = ["cleanup-temp"]
go-args = ["-trimpath"]
application-args = []
output = "./build/admin"

[commands.build.environment]
CGO_ENABLED = "0"
```

| 字段 | 类型 | 默认值 | 作用 |
| --- | --- | --- | --- |
| `before` | 任务名数组 | 空 | 生成成功后、官方命令前执行；对于 `fix`，在 `go fix` 前执行。 |
| `after` | 任务名数组 | 空 | 只在主生命周期成功后执行。 |
| `finally` | 任务名数组 | 空 | 无论主生命周期成功或失败，都按反向依赖顺序执行。 |
| `go-args` | 字符串数组 | 空 | 放在 Profile 和 CLI Go 参数之前。 |
| `application-args` | 字符串数组 | 空 | 默认应用参数，主要用于 `run`。 |
| `environment` | 字符串映射 | 空 | 当前命令的环境覆盖层。 |
| `output` | 项目相对路径 | 空 | 作为 `${command.output}`；对 `build`，CLI 未提供 `-o` 时转成 `go build -o`。 |

每个钩子名都必须引用已声明任务。空配置节和空数组都可省略，不需要为了形式完整而添加。

## `[tools.<name>]`

工具名和任务名必须匹配 `[A-Za-z0-9][A-Za-z0-9._-]*`。

### Go 工具

```toml
[tools.goark-orm]
type = "go"
package = "goark.dev/orm/cmd/goark-orm"
version = "v0.1.0"
install = "auto"
```

- 必须声明 `package` 和精确语义化 `version`。
- `install` 可选 `auto` 或 `manual`。
- 工具安装到 Goark 用户缓存，不写项目或全局 `GOBIN`。
- 自动恢复必须具有匹配锁文件和受信任的项目摘要。

### 系统工具

```toml
[tools.sha256]
type = "system"
command = "sha256sum"
install = "manual"
```

- `command` 是从 `PATH` 查找的命令名，不能是路径或 Shell 表达式。
- Goark 永不安装系统工具。
- `install` 必须是 `manual`。

### 本地工具

```toml
[tools.asset-compiler]
type = "local"
path = "./tools/asset-compiler"
install = "manual"
```

- 符号链接解析后，`path` 必须仍指向项目内可执行文件。
- `install` 必须是 `manual`。

不同工具类型的专属字段不能混用。生命周期细节见[工具与锁定](tools-lock-cache.zh-CN.md)。

## `[tasks.<name>]`

```toml
[tasks.orm-generate]
type = "exec"
tool = "goark-orm"
args = ["generate", "orm", "./..."]
working-directory = "."
depends-on = []
inputs = ["**/*.go", "resource/orm/**/*.xml"]
outputs = ["**/zz_goark_orm_*_gen.go"]
environment-inputs = ["GOARK_ORM_CONFIG"]
timeout = "2m"
cache = true
parallel-safe = false
when = 'profile != "docs"'

[tasks.orm-generate.environment]
GOARK_ORM_CONFIG = "resource/orm/config.toml"
```

| 字段 | 类型 | 默认值 | 作用 |
| --- | --- | --- | --- |
| `type` | 枚举 | 必填 | `exec`、`go`、`delete` 或 `group`；用户文件使用内部 `goark-generate` 会失败。 |
| `tool` | 字符串 | 空 | 仅 `exec` 需要，引用 `[tools.<name>]`。 |
| `args` | 字符串数组 | 空 | 不经过 Shell，直接作为参数数组；`go` 任务至少需要一个参数。 |
| `working-directory` | 项目相对路径 | 项目根 | 进程执行使用的已存在目录。 |
| `depends-on` | 任务名数组 | 空 | 必须先完成的上游任务。 |
| `inputs` | 路径/通配模式数组 | 空 | 纳入缓存指纹的文件和目录。 |
| `outputs` | 路径/通配模式数组 | 空 | 输出所有权、冲突检查、缓存校验和删除目标。 |
| `environment-inputs` | 环境变量名数组 | 空 | 对值做哈希并纳入缓存键，但不保存明文。 |
| `environment` | 字符串映射 | 空 | 任务环境覆盖，值支持变量替换。 |
| `timeout` | Go duration 字符串 | `execution.default-timeout` | 单任务超时；`0` 表示使用全局默认，不能为负数。 |
| `cache` | 布尔值 | `false` | 启用输出校验缓存，必须同时声明非空 `inputs` 和 `outputs`。 |
| `parallel-safe` | 布尔值 | `false` | 允许与其他安全的就绪节点并发执行。 |
| `when` | 表达式字符串 | 始终为真 | 执行前计算的无副作用条件。 |

### 任务类型

| 类型 | 行为 | 关键规则 |
| --- | --- | --- |
| `exec` | 执行声明的外部工具。 | `tool` 必须存在；可执行文件和参数始终分离。 |
| `go` | 使用 `args` 执行官方 `go`。 | 首个参数通常是 Go 子命令。 |
| `delete` | 删除声明输出的匹配项。 | 输出必须位于项目内，禁止缓存。 |
| `group` | 聚合依赖，不启动进程。 | 至少声明一个依赖。 |

## `[profiles.<name>]`

```toml
[profiles.production]
go-args = ["-tags=production", "-trimpath"]
application-args = []

[profiles.production.environment]
GOARK_PROFILES_ACTIVE = "production"
```

| 字段 | 类型 | 默认值 | 作用 |
| --- | --- | --- | --- |
| `go-args` | 字符串数组 | 空 | 位于命令默认参数之后、CLI 参数之前的构建参数。 |
| `application-args` | 字符串数组 | 空 | 位于命令默认参数之后、CLI 参数之前的应用参数。 |
| `environment` | 字符串映射 | 空 | 优先级高于命令环境的覆盖层。 |

使用 `--goark-profile=production` 选择 Profile。选择不存在的 Profile 会失败。这是构建期选择，不会从 Boot 运行期属性 `goark.profiles.active` 自动推导。

## 环境和参数优先级

环境变量按照以下优先级覆盖，越靠上优先级越高：

```text
--goark-env
Profile 环境
命令环境
任务环境
进程环境
```

Go 参数和应用参数数组按以下顺序组装：

```text
命令配置 -> Profile 配置 -> CLI 参数
```

因此，对于“最后一次出现生效”的 Go flag，CLI 仍是最终覆盖层。

环境变量名必须匹配 `[A-Za-z_][A-Za-z0-9_]*`。Windows 上环境名按大小写不敏感处理。

## 变量替换

任务 `args`、`inputs`、`outputs`、`working-directory` 和任务环境值会执行一次变量替换。

| 变量 | 值 |
| --- | --- |
| `${project.root}` | 规范化项目根目录 |
| `${project.name}` | 最终项目名 |
| `${project.module}` | `go.mod` 中的 module path |
| `${profile}` | 当前构建 Profile，未选择时为空 |
| `${command.output}` | 当前命令 `output`，没有值时失败 |
| `${tool:name}` | 已验证的声明工具可执行路径 |
| `${env:NAME}` | 最终声明/进程环境变量值 |

替换不递归。反引号、`$()`、未知变量、缺失环境变量和缺失工具都会失败，不执行任何 Shell 展开。

## `when` 表达式

允许的值为 `profile`、`goos`、`goarch`、`env.NAME`、字符串字面量、`true`、`false`。允许的操作符为 `==`、`!=`、`&&`、`||`、`!` 和括号。

```toml
when = 'profile == "production" && goos == "linux"'
```

引用的 `env.NAME` 必须存在于最终环境。函数调用、命令、数字和任意标识符都会被拒绝。

## 完整案例

```toml
version = 1

[project]
name = "admin"
main = "./cmd/server"
description = "管理服务"

[execution]
max-parallel = 4
fail-fast = true
default-timeout = "5m"

[generate]
patterns = ["./..."]
clean-stale = true

[commands.generate]
before = ["orm-generate"]

[commands.build]
after = ["checksum"]
go-args = ["-trimpath"]
output = "./build/admin"

[commands.test]
go-args = ["-count=1", "./..."]

[tools.goark-orm]
type = "go"
package = "goark.dev/orm/cmd/goark-orm"
version = "v0.1.0"
install = "auto"

[tools.sha256]
type = "system"
command = "sha256sum"
install = "manual"

[tasks.orm-generate]
type = "exec"
tool = "goark-orm"
args = ["generate", "orm", "./..."]
inputs = ["**/*.go", "resource/orm/**/*.xml"]
outputs = ["**/zz_goark_orm_*_gen.go"]
environment-inputs = ["GOARK_ORM_CONFIG"]
cache = true

[tasks.orm-generate.environment]
GOARK_ORM_CONFIG = "resource/orm/config.toml"

[tasks.checksum]
type = "exec"
tool = "sha256"
args = ["${command.output}"]

[profiles.dev]
go-args = ["-tags=dev"]

[profiles.dev.environment]
GOARK_PROFILES_ACTIVE = "dev"

[profiles.production]
go-args = ["-tags=production", "-trimpath"]

[profiles.production.environment]
GOARK_PROFILES_ACTIVE = "production"
```

`sha256sum` 示例只适用于 Linux。跨平台项目应为 Windows 声明其他系统工具，或者通过任务条件隔离平台。

## 校验与安全

执行前会拒绝非法 TOML、不支持的版本、未知字段、非法标识符、项目路径逃逸、符号链接逃逸、缺失依赖、重复依赖、循环、任务输出重叠、工具缺失和锁文件漂移。

环境变量名包含 `PASSWORD`、`PASSWD`、`SECRET`、`TOKEN`、`API_KEY`、`PRIVATE_KEY`、`CREDENTIAL` 等标记时，其值会在计划和诊断输出中脱敏。
