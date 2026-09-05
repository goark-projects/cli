# CI 与离线工作流

[English](ci-workflows.md) | 简体中文

## 仓库文件

应提交 `go.mod`、`go.sum`、`goark.build`；声明工具或使用锁定执行时还应提交 `goark.build.lock`。仓库策略要求跟踪生成源码时，应提交 Goark 生成的 `.go` 文件。不要提交 `.goark/cache`、用户工具缓存或项目信任记录。

## 开发者准备

修改 `goark.build` 后，本地同步并验证：

```bash
goark sync
goark doctor
goark info --json
goark test -race ./...
goark vet ./...
```

锁文件需要包含多平台锁定项时，应在每个支持的 GOOS/GOARCH 上运行 `goark sync`。

## 锁定 CI

```bash
goark sync --locked
goark generate --goark-locked
goark test --goark-locked -count=1 ./...
goark vet --goark-locked ./...
goark build --goark-locked
```

`--goark-locked` 要求当前平台上全部声明工具都有完整锁定项，包括当前命令不可达任务使用的工具。这是最严格的可复现门禁。

## GitHub Actions 示例

```yaml
name: validation

on:
  push:
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - run: go install goark.dev/cli/cmd/goark@latest
      - run: goark sync --locked
      - run: goark test --goark-locked -race -count=1 ./...
      - run: goark vet --goark-locked ./...
      - run: goark build --goark-locked
```

发布流水线应锁定 Goark CLI 版本，不应使用 `@latest`。

## 生成源码漂移

仓库提交生成文件时，应重新生成并要求差异为空：

```bash
goark generate --goark-locked
git diff --exit-code -- '*.go'
```

每次生成都会重建并原子替换 Goark 所有目标，因此可以发现过期或不确定输出。

## 离线执行

```bash
goark tool verify
goark test --goark-offline --goark-locked ./...
goark build --goark-offline --goark-locked
```

断网前应准备 Go module cache 和 Goark 工具缓存，系统工具和本地工具也必须已存在。

`goark sync --offline` 可以根据本机可解析工具更新锁文件，它不是只读操作。离线且不允许更新锁时使用 `goark sync --locked` 或 `goark tool verify`。

## CI 中的 Profile

构建 Profile 与 Boot 运行时 Profile 彼此独立：

```bash
goark build --goark-profile=production --goark-locked
goark run --goark-profile=production --goark.profiles.active=production
```

`--goark-profile` 控制 `goark.build` 的 Go 参数、任务条件和环境覆盖。`GOARK_PROFILES_ACTIVE` 或 `--goark.profiles.active` 控制 Boot 配置文件和运行期 Bean。

## 跨平台规则

- 不要无条件使用只存在于某个操作系统的 `system` 工具。
- 适合时使用平台 `when` 条件或拆分平台任务。
- 条件会跳过任务执行，但如果 `exec` 任务从生命周期可达，准备阶段仍可能要求其工具存在。
- 存在等价包时，优先使用 Go 工具实现跨平台行为。
- 在每个支持平台生成并提交锁定项。

## Dry-run 门禁

```bash
goark info --json > goark-info.json
goark build --goark-profile=production --goark-dry-run
```

dry-run 会校验配置、路径、任务图、锁要求和计划，但不写入、不启动进程。可达外部工具仍要求有效锁和本机可解析的已验证可执行文件，才能报告最终路径。
