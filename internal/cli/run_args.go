package cli

import (
	"fmt"
	"strings"

	"goark.dev/cli/internal/buildplan"
)

// runArguments 保存 go run 与 Goark 编译前阶段的参数边界。
type runArguments struct {
	GoArguments          []string
	PropertyArguments    []string
	ApplicationArguments []string
	Target               string
	TargetExplicit       bool
	Control              buildplan.Control
}

var goBuildFlagsWithValue = map[string]struct{}{
	"-C":             {},
	"-asmflags":      {},
	"-buildmode":     {},
	"-buildvcs":      {},
	"-compiler":      {},
	"-covermode":     {},
	"-coverpkg":      {},
	"-exec":          {},
	"-gccgoflags":    {},
	"-gcflags":       {},
	"-installsuffix": {},
	"-ldflags":       {},
	"-mod":           {},
	"-modfile":       {},
	"-o":             {},
	"-overlay":       {},
	"-p":             {},
	"-pkgdir":        {},
	"-pgo":           {},
	"-tags":          {},
	"-toolexec":      {},
}

func parseRunArguments(args []string) (runArguments, error) {
	plan := runArguments{}
	afterTarget := false
	goFiles := false
	applicationOnly := false

	for index := 0; index < len(args); index++ {
		arg := args[index]
		if applicationOnly {
			plan.ApplicationArguments = append(plan.ApplicationArguments, arg)
			continue
		}
		if arg == "--" {
			applicationOnly = true
			continue
		}
		if !afterTarget {
			handled, err := applyRunControlArgument(&plan, arg)
			if err != nil {
				return runArguments{}, err
			}
			if handled {
				continue
			}
		}
		if strings.HasPrefix(arg, "-D") {
			if err := validateSystemPropertyArgument(arg); err != nil {
				return runArguments{}, err
			}
			plan.PropertyArguments = append(plan.PropertyArguments, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			plan.PropertyArguments = append(plan.PropertyArguments, arg)
			if !strings.Contains(arg, "=") && index+1 < len(args) && args[index+1] != "--" && !strings.HasPrefix(args[index+1], "-") {
				index++
				plan.PropertyArguments = append(plan.PropertyArguments, args[index])
			}
			continue
		}
		if !afterTarget && strings.HasPrefix(arg, "-") {
			plan.GoArguments = append(plan.GoArguments, arg)
			if goBuildFlagConsumesValue(arg) {
				if index+1 >= len(args) {
					return runArguments{}, fmt.Errorf("Go 构建参数 %s 缺少值", arg)
				}
				index++
				plan.GoArguments = append(plan.GoArguments, args[index])
			}
			continue
		}
		if !afterTarget {
			plan.Target = arg
			plan.TargetExplicit = true
			plan.GoArguments = append(plan.GoArguments, arg)
			afterTarget = true
			goFiles = strings.HasSuffix(arg, ".go")
			continue
		}
		if goFiles && strings.HasSuffix(arg, ".go") {
			plan.GoArguments = append(plan.GoArguments, arg)
			continue
		}
		plan.ApplicationArguments = append(plan.ApplicationArguments, arg)
	}

	return plan, nil
}

func applyRunControlArgument(plan *runArguments, arg string) (bool, error) {
	return buildplan.ApplyControlArgument(&plan.Control, arg)
}

func validateSystemPropertyArgument(arg string) error {
	body := strings.TrimPrefix(arg, "-D")
	key, _, found := strings.Cut(body, "=")
	if !found || strings.TrimSpace(key) == "" {
		return fmt.Errorf("系统属性必须使用 -Dkey=value 格式: %s", arg)
	}
	return nil
}

func goBuildFlagConsumesValue(arg string) bool {
	if strings.Contains(arg, "=") {
		return false
	}
	_, ok := goBuildFlagsWithValue[arg]
	return ok
}

// GoRunArguments 返回传给 go run 的最终参数，不包含 run 子命令本身。
func (p runArguments) GoRunArguments() []string {
	args := make([]string, 0, len(p.GoArguments)+len(p.PropertyArguments)+len(p.ApplicationArguments))
	args = append(args, p.GoArguments...)
	args = append(args, p.PropertyArguments...)
	args = append(args, p.ApplicationArguments...)
	return args
}

// WithResolvedTarget 为零配置运行计划补充自动发现的 main package。
func (p runArguments) WithResolvedTarget(target string) runArguments {
	if p.TargetExplicit {
		return p
	}
	p.Target = target
	p.GoArguments = append(p.GoArguments, target)
	return p
}
