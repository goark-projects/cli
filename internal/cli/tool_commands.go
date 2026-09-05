package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"goark.dev/cli/internal/buildplan"
	"goark.dev/cli/internal/tooling"
	"goark.dev/cli/internal/toolservice"
)

func (c Command) runSync(args []string) int {
	if isHelpOnly(args) {
		c.printSyncHelp(c.Out)
		return 0
	}
	options, err := parseSyncOptions(args)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	service, err := c.projectToolService()
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	locked, err := service.Sync(c.Context, options)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return executionErrorCode(err)
	}
	if options.Locked {
		_, _ = fmt.Fprintf(c.Out, "verified %d tools\n", len(locked.Tools))
	} else {
		_, _ = fmt.Fprintf(c.Out, "synchronized %d tools\n", len(locked.Tools))
	}
	return 0
}

func parseSyncOptions(args []string) (toolservice.SyncOptions, error) {
	var options toolservice.SyncOptions
	for _, argument := range args {
		switch argument {
		case "--locked":
			if options.Locked {
				return toolservice.SyncOptions{}, fmt.Errorf("重复参数 --locked")
			}
			options.Locked = true
		case "--offline":
			if options.Offline {
				return toolservice.SyncOptions{}, fmt.Errorf("重复参数 --offline")
			}
			options.Offline = true
		default:
			return toolservice.SyncOptions{}, fmt.Errorf("goark sync 仅支持 --locked 和 --offline: %s", argument)
		}
	}
	return options, nil
}

func (c Command) runTools(args []string) int {
	if isHelpOnly(args) {
		c.printToolsHelp(c.Out)
		return 0
	}
	jsonOutput := false
	if len(args) > 0 {
		if len(args) != 1 || args[0] != "--json" {
			_, _ = fmt.Fprintf(c.Err, "goark tools 仅支持 --json: %s\n", strings.Join(args, " "))
			return 2
		}
		jsonOutput = true
	}
	service, err := c.projectToolService()
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	statuses := service.Statuses(c.Context)
	if jsonOutput {
		encoder := json.NewEncoder(c.Out)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(statuses); err != nil {
			_, _ = fmt.Fprintf(c.Err, "输出工具状态失败: %v\n", err)
			return 1
		}
		return 0
	}
	for _, status := range statuses {
		_, _ = fmt.Fprintf(c.Out, "%s\t%s\t%s\n", status.Name, status.Type, status.Status)
	}
	return 0
}

func (c Command) runTool(args []string) int {
	if isHelpOnly(args) {
		c.printToolHelp(c.Out)
		return 0
	}
	if len(args) == 1 && args[0] == "verify" {
		service, err := c.projectToolService()
		if err != nil {
			_, _ = fmt.Fprintln(c.Err, err)
			return 2
		}
		locked, err := service.Verify(c.Context)
		if err != nil {
			_, _ = fmt.Fprintln(c.Err, err)
			return executionErrorCode(err)
		}
		_, _ = fmt.Fprintf(c.Out, "verified %d tools\n", len(locked.Tools))
		return 0
	}
	if len(args) == 2 && args[0] == "install" {
		service, err := c.projectToolService()
		if err != nil {
			_, _ = fmt.Fprintln(c.Err, err)
			return 2
		}
		if _, ok := service.Document.Tools[args[1]]; !ok {
			_, _ = fmt.Fprintf(c.Err, "工具 %q 不存在\n", args[1])
			return 2
		}
		resolved, err := service.Install(c.Context, args[1], false)
		if err != nil {
			_, _ = fmt.Fprintln(c.Err, err)
			return executionErrorCode(err)
		}
		_, _ = fmt.Fprintf(c.Out, "installed %s: %s\n", resolved.Name, resolved.Path)
		return 0
	}
	_, _ = fmt.Fprint(c.Err, "Usage:\n  goark tool install <name>\n  goark tool verify\n")
	return 2
}

func (c Command) projectToolService() (toolservice.Service, error) {
	project, err := c.resolveProjectMetadata(c.Dir)
	if err != nil {
		return toolservice.Service{}, err
	}
	plan, err := buildplan.Create(project.Build, "tools", buildplan.Control{}, nil, nil, nil, c.environment())
	if err != nil {
		return toolservice.Service{}, err
	}
	cacheDirectory := c.ToolCacheDir
	if cacheDirectory == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return toolservice.Service{}, fmt.Errorf("解析用户缓存目录失败: %w", err)
		}
		cacheDirectory = filepath.Join(userCache, "goark", "tools")
	}
	trust, err := c.projectTrustStore()
	if err != nil {
		return toolservice.Service{}, err
	}
	return toolservice.Service{
		Root: project.Root, Document: project.Build, Environment: plan.Environment,
		Manager: tooling.NewManager(project.Root, cacheDirectory, plan.Environment), Trust: trust,
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	}, nil
}

func (c Command) printSyncHelp(writer io.Writer) {
	_, _ = fmt.Fprint(writer, "Usage:\n  goark sync [--locked] [--offline]\n")
}

func (c Command) printToolsHelp(writer io.Writer) {
	_, _ = fmt.Fprint(writer, "Usage:\n  goark tools [--json]\n")
}

func (c Command) printToolHelp(writer io.Writer) {
	_, _ = fmt.Fprint(writer, "Usage:\n  goark tool install <name>\n  goark tool verify\n")
}
