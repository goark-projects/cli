package buildplan

import (
	"fmt"
	"runtime"
	"sort"
	"strings"

	"goark.dev/cli/internal/buildspec"
)

const RedactedValue = "******"

// Plan 是配置层合并后的不可变命令执行输入。
type Plan struct {
	Command              string
	Profile              string
	GoArguments          []string
	PropertyArguments    []string
	ApplicationArguments []string
	Environment          map[string]string
	Output               string
	Control              Control
}

// Create 按固定优先级创建命令执行计划。
func Create(
	document buildspec.Document,
	commandName string,
	control Control,
	cliGoArguments []string,
	propertyArguments []string,
	cliApplicationArguments []string,
	processEnvironment []string,
) (Plan, error) {
	command := document.Commands[commandName]
	profile, err := selectProfile(document, control.Profile)
	if err != nil {
		return Plan{}, err
	}

	goArguments := make([]string, 0, len(command.GoArgs)+len(profile.GoArgs)+len(cliGoArguments))
	goArguments = append(goArguments, command.GoArgs...)
	goArguments = append(goArguments, profile.GoArgs...)
	goArguments = append(goArguments, cliGoArguments...)

	applicationArguments := make([]string, 0, len(command.ApplicationArgs)+len(profile.ApplicationArgs)+len(cliApplicationArguments))
	applicationArguments = append(applicationArguments, command.ApplicationArgs...)
	applicationArguments = append(applicationArguments, profile.ApplicationArgs...)
	applicationArguments = append(applicationArguments, cliApplicationArguments...)

	environment := buildEnvironment(processEnvironment, command.Environment, profile.Environment, control.Environment)

	return Plan{
		Command:              commandName,
		Profile:              control.Profile,
		GoArguments:          goArguments,
		PropertyArguments:    append([]string(nil), propertyArguments...),
		ApplicationArguments: applicationArguments,
		Environment:          environment,
		Output:               command.Output,
		Control:              cloneControl(control),
	}, nil
}

func selectProfile(document buildspec.Document, name string) (buildspec.Profile, error) {
	if name == "" {
		return buildspec.Profile{}, nil
	}
	profile, ok := document.Profiles[name]
	if !ok {
		return buildspec.Profile{}, fmt.Errorf("goark.build 未定义 Profile %q", name)
	}
	return profile, nil
}

func buildEnvironment(process []string, layers ...map[string]string) map[string]string {
	values := make(map[string]string, len(process))
	canonicalNames := make(map[string]string, len(process))
	set := func(name string, value string) {
		canonical := name
		if runtime.GOOS == "windows" {
			canonical = strings.ToUpper(name)
		}
		if previous, ok := canonicalNames[canonical]; ok && previous != name {
			delete(values, previous)
		}
		values[name] = value
		canonicalNames[canonical] = name
	}
	for _, entry := range process {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name != "" {
			set(name, value)
		}
	}
	for _, layer := range layers {
		for name, value := range layer {
			set(name, value)
		}
	}
	return values
}

func mergeEnvironment(target map[string]string, source map[string]string) {
	for name, value := range source {
		target[name] = value
	}
}

func cloneControl(control Control) Control {
	cloned := control
	cloned.Environment = make(map[string]string, len(control.Environment))
	mergeEnvironment(cloned.Environment, control.Environment)
	return cloned
}

// EnvironmentList 返回适合 os/exec 的稳定环境数组。
func (p Plan) EnvironmentList() []string {
	names := make([]string, 0, len(p.Environment))
	for name := range p.Environment {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, name+"="+p.Environment[name])
	}
	return result
}

// RedactEnvironment 创建可安全输出的环境副本。
func RedactEnvironment(environment map[string]string) map[string]string {
	result := make(map[string]string, len(environment))
	for name, value := range environment {
		if isSecretName(name) {
			value = RedactedValue
		}
		result[name] = value
	}
	return result
}

func isSecretName(name string) bool {
	upper := strings.ToUpper(name)
	for _, marker := range []string{"PASSWORD", "PASSWD", "SECRET", "TOKEN", "API_KEY", "PRIVATE_KEY", "CREDENTIAL"} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}
