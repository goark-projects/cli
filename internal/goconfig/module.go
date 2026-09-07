// Package goconfig 通过 Go 命令读取实际生效的工具链配置。
package goconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"goark.dev/cli/internal/processrun"
)

// ModuleEnvironment 返回当前目录及子进程环境下的模块模式配置。
func ModuleEnvironment(
	ctx context.Context,
	runner processrun.Runner,
	dir string,
	environment []string,
) (flags, workspace string, err error) {
	if runner == nil {
		runner = processrun.OSRunner{}
	}
	var output, diagnostic bytes.Buffer
	err = runner.Run(processrun.Request{
		Context: ctx, Name: "go", Args: []string{"env", "-json", "GOFLAGS", "GOWORK"},
		Dir: dir, Env: environment, Out: &output, Err: &diagnostic,
	})
	if err != nil {
		return "", "", fmt.Errorf("读取 Go 模块配置失败: %w: %s", err, &diagnostic)
	}
	var values struct {
		Flags     *string `json:"GOFLAGS"`
		Workspace *string `json:"GOWORK"`
	}
	if err := json.Unmarshal(output.Bytes(), &values); err != nil {
		return "", "", fmt.Errorf("解析 Go 模块配置失败: %w", err)
	}
	if values.Flags == nil || values.Workspace == nil {
		return "", "", fmt.Errorf("读取的 Go 模块配置缺少 GOFLAGS 或 GOWORK")
	}
	return *values.Flags, *values.Workspace, nil
}
