# 项目创建

[English](project-creation.md) | 简体中文

## 命令

```text
goark new [-type app|web] [-module <module-path>] [-dir <path>] <name>
```

`<name>` 是唯一位置参数，必须放在 flags 后面。

| 参数 | 默认值 | 作用 |
| --- | --- | --- |
| `-type app\|web` | `app` | 选择非 Web Boot 应用或 Web 服务。 |
| `-module <path>` | 项目名 | 设置写入 `go.mod` 的 `module` 指令。 |
| `-dir <path>` | 当前目录 | 选择输出目录；CLI 不会自动追加项目名。 |
| `-force` | `false` | 目标已存在时，覆盖脚手架负责的文件。 |

旧命令 `goark new app --web` 被明确禁止，不提供兼容。

## `app` 场景

`app` 适用于工作进程、定时任务、消息消费者、命令应用，以及需要 Boot 配置和依赖注入但不需要 HTTP 服务的程序。

```bash
mkdir billing-worker
cd billing-worker
goark new -module example.com/platform/billing-worker billing-worker
```

生成项目包含：

- 使用 `//goark:application` 和 `//goark:configuration` 声明应用。
- 生成带注解的 Service 接口、实现和注入元数据。
- `goark.dev/gbc-log` 自动配置。
- 极简 `main.go`，Boot 启动和关闭由生成代码负责。
- 指向 `./cmd/app` 的精简 `goark.build`。

## `web` 场景

`web` 适用于 HTTP API、MVC 应用、管理服务和静态资源服务。

```bash
goark new -type web \
  -module example.com/platform/admin \
  -dir admin \
  admin
```

Web 模板额外包含：

- Arkarta 和 Arkhos 依赖。
- `goark.dev/gbc-arkhos` 与 `goark.dev/gbc-web` 自动配置。
- 返回 `{"status":"UP"}` 的 `GET /healthz`。
- 由注解生成 Controller、路由、Service 注入和应用生命周期。
- `resource/static` 下的静态资源。
- 由中断或终止信号触发的优雅关闭。
- 指向 `./cmd/server` 的 `goark.build`。

## 已存在的目录

未指定 `-force` 时，生成器会先执行完整预检。任何目标文件已存在都会导致生成在写入前失败。指定 `-force` 后，脚手架会使用当前完整模板覆盖对应目标，但不会删除无关文件。

只有目录已纳入版本控制或具备其他恢复方式时才应使用 `-force`：

```bash
goark new -type web -module example.com/admin -dir . -force admin
```

## 生成后操作

```bash
goark generate
go mod tidy
goark info
goark run
```

生成的 `resource/app.yml` 只包含 `goark.application.name`。日志、Arkhos 默认监听地址和
Web 出站客户端均沿用框架默认值，应用需要覆盖时再显式配置。`goark.web.client.*` 配置的
是出站 HTTP 客户端，与嵌入式服务端无关。构建期编排写入 `goark.build`。
