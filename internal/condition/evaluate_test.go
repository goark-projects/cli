package condition

import (
	"runtime"
	"strings"
	"testing"
)

func TestEvaluate_whenExpressionIsValid_shouldReturnBoolean(t *testing.T) {
	values := Values{
		Profile:     "production",
		GOOS:        runtime.GOOS,
		GOARCH:      runtime.GOARCH,
		Environment: map[string]string{"FEATURE": "enabled", "EMPTY": ""},
	}
	tests := []struct {
		expression string
		want       bool
	}{
		{expression: "", want: true},
		{expression: "true", want: true},
		{expression: "false", want: false},
		{expression: `profile == "production"`, want: true},
		{expression: `profile != "dev" && env.FEATURE == "enabled"`, want: true},
		{expression: `!(goos == "unsupported") && (goarch == "` + runtime.GOARCH + `" || false)`, want: true},
		{expression: `env.EMPTY == ""`, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.expression, func(t *testing.T) {
			got, err := Evaluate(tt.expression, values)
			if err != nil {
				t.Fatalf("求值失败: %v", err)
			}
			if got != tt.want {
				t.Fatalf("结果 = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestEvaluate_whenExpressionIsInvalid_shouldReject(t *testing.T) {
	tests := []string{
		"profile",
		`profile = "dev"`,
		`unknown == "value"`,
		`env.MISSING == "value"`,
		`profile == 1`,
		`profile == "dev" trailing`,
		`(profile == "dev"`,
		`exec("whoami")`,
		"`whoami`",
	}
	for _, expression := range tests {
		t.Run(expression, func(t *testing.T) {
			_, err := Evaluate(expression, Values{})
			if err == nil {
				t.Fatalf("非法表达式 %q 必须失败", expression)
			}
		})
	}
}

func TestEvaluate_whenSyntaxFails_shouldIncludePosition(t *testing.T) {
	_, err := Evaluate(`profile == "dev" && )`, Values{})
	if err == nil || !strings.Contains(err.Error(), "位置") {
		t.Fatalf("错误 = %v", err)
	}
}

func TestEvaluate_whenWindowsEnvironmentNameUsesDifferentCase_shouldResolve(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅适用于 Windows 环境名语义")
	}
	got, err := Evaluate(`env.PATH == "configured"`, Values{Environment: map[string]string{"Path": "configured"}})
	if err != nil || !got {
		t.Fatalf("条件结果 = %t, err=%v", got, err)
	}
}
