# 工具、锁文件、信任与缓存

[English](tools-lock-cache.md) | 简体中文

## 工具来源

Goark 根据 `goark.build` 的显式声明解析外部可执行文件。任务中永远不存在 Shell 命令字符串。

| 类型 | 声明字段 | 解析方式 | 安装方式 |
| --- | --- | --- | --- |
| `go` | `package`、精确 `version`、`install` | Goark 隔离用户缓存 | 仅在允许时自动安装，或使用 `tool install` 显式安装 |
| `system` | `command`、`install = "manual"` | 当前最终 `PATH` | 永不安装 |
| `local` | 项目相对 `path`、`install = "manual"` | 项目内规范化路径 | 永不安装 |

普通命令只解析从当前生命周期可达任务所需的工具。未使用的声明工具不会阻塞普通命令，除非 `--goark-locked` 要求完整声明验证。

## 推荐设置流程

新增或修改工具声明后：

```bash
goark sync
goark tool verify
git add goark.build goark.build.lock
```

锁文件应提交。用户工具缓存和本地信任记录不应提交。

## `goark sync`

### 普通模式

```bash
goark sync
```

普通同步流程：

1. 对 `goark.build` 精确字节计算摘要。
2. 存在旧锁文件时读取已有锁定项。
3. 为当前 GOOS/GOARCH 解析全部声明工具。
4. 自动安装声明为 `install = "auto"` 的缺失 Go 工具。
5. 原子写入规范化 `goark.build.lock`。
6. 为规范化项目根目录和构建文件摘要记录本地信任。

其他平台锁定项会保留，当前平台锁定项会被替换。

### 锁定模式

```bash
goark sync --locked
```

锁定模式只验证。现有锁文件必须匹配当前构建摘要、声明、平台锁定项、可执行身份和 SHA-256。它不会安装工具或更新锁文件。

### 离线模式

```bash
goark sync --offline
```

离线模式禁止网络访问和安装，所有工具必须已能在本机解析。解析成功后，`sync` 仍可能更新锁文件和信任记录。需要同时禁止写入时使用 `sync --locked`。

## `goark.build.lock`

锁文件使用严格 TOML、UTF-8 无 BOM 和仅 LF，由 Goark 生成，不应手工编辑。

顶层字段：

| 字段 | 含义 |
| --- | --- |
| `version` | 锁格式版本，当前为 `1`。 |
| `build-sha256` | `goark.build` 精确内容的 SHA-256。 |
| `[[tools]]` | 稳定排序的平台工具锁定项。 |

每个工具锁定项记录：

- `name`、`type`、`goos`、`goarch`。
- 规范化逻辑路径或解析路径 `path`。
- 可执行文件 `sha256`。
- 对 Go 工具，还记录声明 package/version，以及从可执行构建信息读取的 module path、module version、module checksum。

锁定项与平台相关。团队应在每个支持的操作系统/架构运行 `goark sync` 并提交合并结果。

## 项目信任

信任属于本机状态，位于操作系统用户配置目录下的 `goark/trust`。记录绑定：

- 符号链接解析后的规范化项目根目录。
- `goark.build` 精确 SHA-256。

普通 `sync` 或显式 `tool install` 成功后建立信任。编辑 `goark.build` 会让旧记录失效，直到再次同步成功。

生命周期只有同时满足以下条件，才允许自动恢复缺失或漂移的 Go 工具：

- 工具为 `type = "go"` 且 `install = "auto"`。
- 锁文件完整并匹配声明和构建摘要。
- 信任记录匹配规范化根目录和摘要。
- 未启用 dry-run 或 offline。
- 恢复后的可执行文件精确匹配锁定元数据和摘要。

系统工具和本地工具永不自动恢复。

## 工具状态

```bash
goark tools
goark tools --json
```

| 状态 | 含义 |
| --- | --- |
| `ready` | 工具可解析并匹配当前平台锁定项。 |
| `missing` | 本机无法解析工具。 |
| `unlocked` | 工具可解析，但锁文件缺失或不匹配 `goark.build`。 |
| `drift` | 声明、元数据或可执行摘要与锁不一致。 |
| `error` | 构建摘要或其他状态检查失败。 |

`tools` 是只读操作，永不安装。

## 显式安装与验证

```bash
goark tool install goark-orm
goark tool verify
```

`tool install <name>` 强制安装/解析选定声明工具，只更新该工具的当前平台锁定项，并刷新信任。`tool verify` 不安装、不写入，验证全部声明工具。

## Go 工具缓存

Go 工具安装在操作系统用户缓存目录的 `goark/tools/go/<key>/bin` 下。key 包含 package、精确版本、GOOS 和 GOARCH。安装使用临时目录、每工具跨进程锁和原子发布，不修改项目或全局 `GOBIN`。

## 任务缓存

任务缓存清单位于：

```text
.goark/cache/tasks/<task>/<fingerprint>.json
```

缓存必须显式启用：

```toml
[tasks.generate-schema]
type = "go"
args = ["run", "./cmd/schema"]
inputs = ["schema/**/*.json", "cmd/schema/**/*.go"]
outputs = ["internal/schema/schema_gen.go"]
environment-inputs = ["SCHEMA_MODE"]
cache = true
```

缓存指纹包含：

- 规范化任务名和任务定义。
- 输入路径、权限模式和内容 SHA-256。
- 适用时的锁定工具项。
- Go 版本、GOOS、GOARCH 和构建标签。
- 当前构建 Profile。
- 声明环境输入值的 SHA-256，不保存明文。
- 上游依赖输出摘要或跳过状态。

只有每个声明输出都存在，并且当前路径、权限模式和内容摘要与清单完全一致，才能命中。损坏的缓存清单视为未命中，缓存清单采用原子写入。

## 缓存设计规则

- `cache = true` 必须同时声明非空 `inputs` 和 `outputs`。
- 所有能影响输出的文件和环境值都必须声明。
- 网络、时钟或机器相关任务不应缓存，除非这些依赖已转化为显式输入。
- 避免输出重叠；任务图会在执行前拒绝潜在冲突。
- 缓存不会恢复已删除输出。输出缺失或变化会导致未命中并重新执行任务。

## 清理

预览并删除声明的命令输出、任务输出和项目任务缓存：

```bash
goark clean --goark-dry-run
goark clean
```

`clean` 不会删除用户 Go 工具缓存、锁文件或信任记录。
