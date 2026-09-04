package expand

import (
	"strings"
	"testing"
)

func TestString_whenVariablesAreAllowed_shouldExpandOnce(t *testing.T) {
	values := Values{
		ProjectRoot:   "/workspace/app",
		ProjectName:   "admin",
		ProjectModule: "example.com/admin",
		Profile:       "production",
		CommandOutput: "build/admin",
		Tools:         map[string]string{"goark-orm": "/cache/goark-orm"},
		Environment:   map[string]string{"CONFIG": "resource/config.toml", "LITERAL": "${profile}"},
	}
	input := "${project.root}|${project.name}|${project.module}|${profile}|${command.output}|${tool:goark-orm}|${env:CONFIG}|${env:LITERAL}"
	result, err := String(input, values)
	if err != nil {
		t.Fatalf("替换变量失败: %v", err)
	}
	want := "/workspace/app|admin|example.com/admin|production|build/admin|/cache/goark-orm|resource/config.toml|${profile}"
	if result != want {
		t.Fatalf("结果 = %q, want %q", result, want)
	}
}

func TestString_whenExpressionIsNotAllowed_shouldReject(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "unknown variable", input: "${project.version}"},
		{name: "missing environment", input: "${env:MISSING}"},
		{name: "missing tool", input: "${tool:missing}"},
		{name: "shell substitution", input: "$(whoami)"},
		{name: "backticks", input: "`whoami`"},
		{name: "unterminated", input: "${profile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := String(tt.input, Values{})
			if err == nil {
				t.Fatalf("表达式 %q 必须失败", tt.input)
			}
		})
	}
}

func TestStrings_whenOneValueFails_shouldIncludeArgumentIndex(t *testing.T) {
	_, err := Strings([]string{"valid", "${env:MISSING}"}, Values{})
	if err == nil || !strings.Contains(err.Error(), "参数 2") {
		t.Fatalf("错误 = %v", err)
	}
}
