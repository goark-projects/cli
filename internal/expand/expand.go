package expand

import (
	"fmt"
	"strings"

	"goark.dev/cli/internal/envutil"
)

// Values 保存白名单变量的解析上下文。
type Values struct {
	ProjectRoot   string
	ProjectName   string
	ProjectModule string
	Profile       string
	CommandOutput string
	Tools         map[string]string
	Environment   map[string]string
}

// String 对单个参数执行一次白名单变量替换。
func String(input string, values Values) (string, error) {
	if strings.Contains(input, "$(") || strings.ContainsRune(input, '`') {
		return "", fmt.Errorf("禁止 Shell 命令替换")
	}
	first := strings.Index(input, "${")
	if first < 0 {
		return input, nil
	}

	var result strings.Builder
	result.Grow(len(input))
	remaining := input
	for {
		start := strings.Index(remaining, "${")
		if start < 0 {
			result.WriteString(remaining)
			return result.String(), nil
		}
		result.WriteString(remaining[:start])
		tail := remaining[start+2:]
		end := strings.IndexByte(tail, '}')
		if end < 0 {
			return "", fmt.Errorf("变量表达式缺少右花括号")
		}
		expression := tail[:end]
		value, err := resolve(expression, values)
		if err != nil {
			return "", err
		}
		result.WriteString(value)
		remaining = tail[end+1:]
	}
}

// Strings 对参数数组逐项替换并保留原始顺序。
func Strings(inputs []string, values Values) ([]string, error) {
	result := make([]string, len(inputs))
	for index, input := range inputs {
		value, err := String(input, values)
		if err != nil {
			return nil, fmt.Errorf("替换参数 %d 失败: %w", index+1, err)
		}
		result[index] = value
	}
	return result, nil
}

func resolve(expression string, values Values) (string, error) {
	switch expression {
	case "project.root":
		return values.ProjectRoot, nil
	case "project.name":
		return values.ProjectName, nil
	case "project.module":
		return values.ProjectModule, nil
	case "profile":
		return values.Profile, nil
	case "command.output":
		if values.CommandOutput == "" {
			return "", fmt.Errorf("变量 ${command.output} 没有可用值")
		}
		return values.CommandOutput, nil
	}
	if name, ok := strings.CutPrefix(expression, "tool:"); ok {
		value, exists := values.Tools[name]
		if !exists || name == "" {
			return "", fmt.Errorf("未知工具变量 ${tool:%s}", name)
		}
		return value, nil
	}
	if name, ok := strings.CutPrefix(expression, "env:"); ok {
		value, exists := envutil.Lookup(values.Environment, name)
		if !exists || name == "" {
			return "", fmt.Errorf("环境变量 %q 未定义", name)
		}
		return value, nil
	}
	return "", fmt.Errorf("不支持变量 ${%s}", expression)
}
