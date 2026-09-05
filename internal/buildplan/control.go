package buildplan

import (
	"fmt"
	"regexp"
	"strings"
)

var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Control 保存 Goark 自身的控制参数，不包含任何 Go 或应用参数。
type Control struct {
	Profile     string
	DryRun      bool
	Offline     bool
	Locked      bool
	Environment map[string]string
}

// ParseControlArguments 从普通命令参数中提取 Goark 控制参数。
func ParseControlArguments(args []string) ([]string, Control, error) {
	control := Control{Environment: make(map[string]string)}
	remaining := make([]string, 0, len(args))
	passthrough := false
	for _, argument := range args {
		if passthrough {
			remaining = append(remaining, argument)
			continue
		}
		if argument == "--" || argument == "-args" {
			passthrough = true
			remaining = append(remaining, argument)
			continue
		}
		handled, err := ApplyControlArgument(&control, argument)
		if err != nil {
			return nil, Control{}, err
		}
		if !handled {
			remaining = append(remaining, argument)
		}
	}
	return remaining, control, nil
}

// ApplyControlArgument 解析一个可能的 Goark 控制参数。
func ApplyControlArgument(control *Control, argument string) (bool, error) {
	if control.Environment == nil {
		control.Environment = make(map[string]string)
	}
	switch argument {
	case "--goark-dry-run":
		control.DryRun = true
		return true, nil
	case "--goark-offline":
		control.Offline = true
		return true, nil
	case "--goark-locked":
		control.Locked = true
		return true, nil
	case "--goark-no-generate", "--goark-generate-only":
		return false, fmt.Errorf("已删除参数 %s；需要原始 Go 行为时使用 goark go ...", argument)
	}
	if strings.HasPrefix(argument, "--goark-profile=") {
		profile := strings.TrimPrefix(argument, "--goark-profile=")
		if !validIdentifier(profile) {
			return false, fmt.Errorf("无效 Goark Profile: %q", profile)
		}
		control.Profile = profile
		return true, nil
	}
	if strings.HasPrefix(argument, "--goark-env=") {
		assignment := strings.TrimPrefix(argument, "--goark-env=")
		name, value, ok := strings.Cut(assignment, "=")
		if !ok || !environmentNamePattern.MatchString(name) {
			return false, fmt.Errorf("Goark 环境变量必须使用 --goark-env=KEY=VALUE 格式: %s", argument)
		}
		SetEnvironment(control.Environment, name, value)
		return true, nil
	}
	if strings.HasPrefix(argument, "--goark-") {
		return false, fmt.Errorf("未知 Goark 参数: %s", argument)
	}
	return false, nil
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}
