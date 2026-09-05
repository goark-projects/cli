package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"goark.dev/cli/internal/buildplan"
	"goark.dev/cli/internal/taskview"
	"goark.dev/cli/internal/toolservice"
	"golang.org/x/mod/modfile"
)

type infoReport struct {
	CLIVersion string               `json:"cliVersion"`
	Go         infoGo               `json:"go"`
	Project    infoProject          `json:"project"`
	Profile    string               `json:"profile"`
	Tools      []toolservice.Status `json:"tools"`
	Tasks      []taskview.Task      `json:"tasks"`
	Generators []infoGenerator      `json:"generators"`
	Cache      infoCache            `json:"cache"`
	Plans      []infoPlan           `json:"plans"`
}

type infoGo struct {
	Version   string `json:"version"`
	Toolchain string `json:"toolchain,omitempty"`
}

type infoProject struct {
	Name        string `json:"name"`
	Module      string `json:"module"`
	Root        string `json:"root"`
	Main        string `json:"main"`
	Description string `json:"description,omitempty"`
}

type infoGenerator struct {
	Name     string   `json:"name"`
	Patterns []string `json:"patterns"`
	Packages int      `json:"packages"`
}

type infoCache struct {
	Directory string `json:"directory"`
	Exists    bool   `json:"exists"`
	Entries   int    `json:"entries"`
}

type infoPlan struct {
	Command              string            `json:"command"`
	GoArguments          []string          `json:"goArguments"`
	ApplicationArguments []string          `json:"applicationArguments"`
	Environment          map[string]string `json:"environment"`
	Before               []string          `json:"before"`
	After                []string          `json:"after"`
	Finally              []string          `json:"finally"`
	Output               string            `json:"output,omitempty"`
}

func (c Command) runInfo(args []string) int {
	if isHelpOnly(args) {
		c.printInfoHelp(c.Out)
		return 0
	}
	jsonOutput, control, err := parseInfoArguments(args)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	project, err := c.resolveProject(c.Dir, nil, nil, true)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	profilePlan, err := buildplan.Create(project.Build, "generate", control, nil, nil, nil, c.environment())
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	project, err = c.resolveProject(c.Dir, nil, discoveryBuildFlags(profilePlan.GoArguments), true)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	report, err := c.createInfoReport(project, control)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 1
	}
	if jsonOutput {
		encoder := json.NewEncoder(c.Out)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(report); err != nil {
			_, _ = fmt.Fprintf(c.Err, "输出 info JSON 失败: %v\n", err)
			return 1
		}
		return 0
	}
	writeInfoText(c.Out, report)
	return 0
}

func parseInfoArguments(args []string) (bool, buildplan.Control, error) {
	control := buildplan.Control{}
	jsonOutput := false
	for _, argument := range args {
		switch {
		case argument == "--json":
			if jsonOutput {
				return false, buildplan.Control{}, fmt.Errorf("goark info 重复参数 --json")
			}
			jsonOutput = true
		case strings.HasPrefix(argument, "--goark-profile="):
			handled, err := buildplan.ApplyControlArgument(&control, argument)
			if err != nil {
				return false, buildplan.Control{}, err
			}
			if !handled {
				return false, buildplan.Control{}, fmt.Errorf("无效 info 参数: %s", argument)
			}
		default:
			return false, buildplan.Control{}, fmt.Errorf("goark info 不支持参数: %s", argument)
		}
	}
	return jsonOutput, control, nil
}

func (c Command) createInfoReport(project goarkProject, control buildplan.Control) (infoReport, error) {
	if err := validateProjectTaskGraph(project); err != nil {
		return infoReport{}, err
	}
	mainTarget, err := project.ResolveRunTarget(effectiveBaseDir(c.Dir))
	if err != nil {
		mainTarget = "unresolved (" + err.Error() + ")"
	}
	goMetadata, err := readGoMetadata(filepath.Join(project.Root, "go.mod"))
	if err != nil {
		return infoReport{}, err
	}
	generated, err := generateProject(project, true)
	if err != nil {
		return infoReport{}, err
	}
	cache, err := inspectCache(project.Root)
	if err != nil {
		return infoReport{}, err
	}
	toolPlan, err := buildplan.Create(project.Build, "tools", control, nil, nil, nil, c.environment())
	if err != nil {
		return infoReport{}, err
	}
	toolService, err := c.newProjectToolService(project, toolPlan.Environment)
	if err != nil {
		return infoReport{}, err
	}
	plans, err := createInfoPlans(project, c.environment(), control)
	if err != nil {
		return infoReport{}, err
	}
	return infoReport{
		CLIVersion: Version, Go: goMetadata,
		Project: infoProject{
			Name: project.ProjectName(), Module: project.ModulePath, Root: project.Root,
			Main: mainTarget, Description: project.Build.Project.Description,
		},
		Tools: toolService.Statuses(c.Context), Tasks: taskview.Snapshot(project.Build.Tasks),
		Generators: []infoGenerator{{Name: "annotations", Patterns: append([]string(nil), project.Build.Generate.Patterns...), Packages: len(generated)}},
		Profile:    control.Profile, Cache: cache, Plans: plans,
	}, nil
}

func readGoMetadata(path string) (infoGo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return infoGo{}, fmt.Errorf("读取 go.mod 失败: %w", err)
	}
	parsed, err := modfile.Parse(path, data, nil)
	if err != nil {
		return infoGo{}, fmt.Errorf("解析 go.mod 失败: %w", err)
	}
	result := infoGo{}
	if parsed.Go != nil {
		result.Version = parsed.Go.Version
	}
	if parsed.Toolchain != nil {
		result.Toolchain = parsed.Toolchain.Name
	}
	return result, nil
}

func inspectCache(root string) (infoCache, error) {
	directory := filepath.Join(root, ".goark", "cache")
	result := infoCache{Directory: directory}
	info, err := os.Stat(directory)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return infoCache{}, fmt.Errorf("检查任务缓存失败: %w", err)
	}
	result.Exists = info.IsDir()
	if !result.Exists {
		return infoCache{}, fmt.Errorf("任务缓存路径不是目录: %s", directory)
	}
	err = filepath.WalkDir(directory, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			result.Entries++
		}
		return nil
	})
	if err != nil {
		return infoCache{}, fmt.Errorf("读取任务缓存失败: %w", err)
	}
	return result, nil
}

func createInfoPlans(project goarkProject, environment []string, control buildplan.Control) ([]infoPlan, error) {
	commands := []string{"build", "fix", "generate", "install", "list", "run", "test", "vet"}
	plans := make([]infoPlan, 0, len(commands))
	for _, name := range commands {
		plan, err := buildplan.Create(project.Build, name, control, nil, nil, nil, environment)
		if err != nil {
			return nil, err
		}
		configuration := project.Build.Commands[name]
		arguments := append([]string(nil), plan.GoArguments...)
		if name != "generate" {
			arguments, err = applyDefaultBuildTarget(project, project.Root, name, arguments)
			if err != nil {
				return nil, err
			}
			arguments = composeEnhancedGoArguments(name, applyCommandOutput(name, arguments, plan.Output))
		}
		plans = append(plans, infoPlan{
			Command: name, GoArguments: arguments,
			ApplicationArguments: append([]string(nil), plan.ApplicationArguments...),
			Environment:          buildplan.RedactEnvironment(lifecycleOverrides(project.Build, plan)),
			Before:               append([]string(nil), configuration.Before...), After: append([]string(nil), configuration.After...),
			Finally: append([]string(nil), configuration.Finally...), Output: plan.Output,
		})
	}
	return plans, nil
}

func writeInfoText(writer io.Writer, report infoReport) {
	profile := report.Profile
	if profile == "" {
		profile = "(none)"
	}
	_, _ = fmt.Fprintf(writer, "Goark CLI: %s\n", report.CLIVersion)
	_, _ = fmt.Fprintf(writer, "Go version: %s\n", report.Go.Version)
	_, _ = fmt.Fprintf(writer, "Project: %s\n", report.Project.Name)
	_, _ = fmt.Fprintf(writer, "Module: %s\n", report.Project.Module)
	_, _ = fmt.Fprintf(writer, "Root: %s\n", report.Project.Root)
	_, _ = fmt.Fprintf(writer, "Main: %s\n", report.Project.Main)
	_, _ = fmt.Fprintf(writer, "Profile: %s\n", profile)
	_, _ = fmt.Fprintf(writer, "Tools: %d\n", len(report.Tools))
	_, _ = fmt.Fprintf(writer, "Tasks: %d\n", len(report.Tasks))
	_, _ = fmt.Fprintf(writer, "Generators: %s\n", report.Generators[0].Name)
	_, _ = fmt.Fprintf(writer, "Generated packages: %d\n", report.Generators[0].Packages)
	_, _ = fmt.Fprintf(writer, "Cache entries: %d\n", report.Cache.Entries)
	_, _ = fmt.Fprintln(writer, "Execution plans:")
	names := make([]string, 0, len(report.Plans))
	for _, plan := range report.Plans {
		names = append(names, plan.Command)
	}
	sort.Strings(names)
	_, _ = fmt.Fprintf(writer, "  %s\n", strings.Join(names, ", "))
}
