package buildplan

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"goark.dev/cli/internal/buildspec"
)

func TestRedactArguments_whenSecretsProvided_shouldPreserveNamesAndHideValues(t *testing.T) {
	arguments := []string{"build", "-Ddb.password=property-secret", "--token=argument-secret", "plain-secret", "value with spaces"}
	environment := map[string]string{"API_TOKEN": "plain-secret"}
	want := []string{"build", "-Ddb.password=******", "--token=******", "******", "value with spaces"}
	got := RedactArguments(arguments, environment)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("脱敏参数 = %#v, want %#v", got, want)
	}
}

func TestParseControlArguments_whenValid_shouldSeparateWithoutReorderingGoArguments(t *testing.T) {
	remaining, control, err := ParseControlArguments([]string{
		"-race",
		"--goark-profile=production",
		"--goark-env=PORT=9090",
		"--goark-env=TOKEN=secret",
		"--goark-offline",
		"--goark-locked",
		"--goark-dry-run",
		"./...",
	})
	if err != nil {
		t.Fatalf("解析控制参数失败: %v", err)
	}
	if !reflect.DeepEqual(remaining, []string{"-race", "./..."}) {
		t.Fatalf("剩余参数 = %#v", remaining)
	}
	if control.Profile != "production" || !control.Offline || !control.Locked || !control.DryRun {
		t.Fatalf("控制参数 = %#v", control)
	}
	if !reflect.DeepEqual(control.Environment, map[string]string{"PORT": "9090", "TOKEN": "secret"}) {
		t.Fatalf("控制环境 = %#v", control.Environment)
	}
}

func TestParseControlArguments_whenPassthroughStarts_shouldLeaveApplicationArgumentsUntouched(t *testing.T) {
	remaining, control, err := ParseControlArguments([]string{"./cmd/app", "--", "--goark-profile=application"})
	if err != nil {
		t.Fatalf("解析控制参数失败: %v", err)
	}
	if control.Profile != "" || !reflect.DeepEqual(remaining, []string{"./cmd/app", "--", "--goark-profile=application"}) {
		t.Fatalf("参数被错误解析: remaining=%#v control=%#v", remaining, control)
	}
}

func TestParseControlArguments_whenWindowsEnvironmentNamesDifferOnlyByCase_shouldUseLastValue(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅适用于 Windows 环境名语义")
	}
	_, control, err := ParseControlArguments([]string{"--goark-env=PATH=first", "--goark-env=Path=second"})
	if err != nil {
		t.Fatalf("解析控制参数失败: %v", err)
	}
	if !reflect.DeepEqual(control.Environment, map[string]string{"Path": "second"}) {
		t.Fatalf("控制环境 = %#v", control.Environment)
	}
}

func TestParseControlArguments_whenInvalid_shouldReject(t *testing.T) {
	tests := []string{
		"--goark-profile=",
		"--goark-env=INVALID",
		"--goark-env==value",
		"--goark-no-generate",
		"--goark-generate-only",
		"--goark-unknown",
	}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			if _, _, err := ParseControlArguments([]string{value}); err == nil {
				t.Fatalf("非法控制参数应失败: %s", value)
			}
		})
	}
}

func TestCreate_whenLayersOverlap_shouldUseDefinedPrecedence(t *testing.T) {
	document := buildspec.Document{
		Commands: map[string]buildspec.Command{
			"build": {
				GoArgs:      []string{"-tags=command", "-p=1"},
				Environment: map[string]string{"SOURCE": "command", "COMMAND": "true"},
				Output:      "./build/app",
			},
		},
		Profiles: map[string]buildspec.Profile{
			"production": {
				GoArgs:          []string{"-tags=profile", "-trimpath"},
				ApplicationArgs: []string{"--profile-argument"},
				Environment:     map[string]string{"SOURCE": "profile", "PROFILE": "true"},
			},
		},
	}
	control := Control{Profile: "production", Environment: map[string]string{"SOURCE": "cli", "CLI": "true"}}

	plan, err := Create(document, "build", control, []string{"-tags=cli", "./..."}, nil, []string{"--cli-argument"}, []string{"SOURCE=process", "PROCESS=true"})
	if err != nil {
		t.Fatalf("创建执行计划失败: %v", err)
	}
	if !reflect.DeepEqual(plan.GoArguments, []string{"-tags=command", "-p=1", "-tags=profile", "-trimpath", "-tags=cli", "./..."}) {
		t.Fatalf("Go 参数 = %#v", plan.GoArguments)
	}
	if !reflect.DeepEqual(plan.ApplicationArguments, []string{"--profile-argument", "--cli-argument"}) {
		t.Fatalf("应用参数 = %#v", plan.ApplicationArguments)
	}
	wantEnvironment := map[string]string{
		"SOURCE": "cli", "PROCESS": "true", "COMMAND": "true", "PROFILE": "true", "CLI": "true",
	}
	if !reflect.DeepEqual(plan.Environment, wantEnvironment) {
		t.Fatalf("环境 = %#v, want %#v", plan.Environment, wantEnvironment)
	}
	if plan.Output != "./build/app" || plan.Profile != "production" {
		t.Fatalf("计划元数据 = %#v", plan)
	}
}

func TestCreate_whenProfileDoesNotExist_shouldReject(t *testing.T) {
	_, err := Create(buildspec.Document{}, "run", Control{Profile: "missing"}, nil, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("错误 = %v", err)
	}
}

func TestPlanEnvironmentList_shouldBeStable(t *testing.T) {
	plan := Plan{Environment: map[string]string{"Z_VALUE": "last", "A_VALUE": "first"}}
	want := []string{"A_VALUE=first", "Z_VALUE=last"}
	if got := plan.EnvironmentList(); !reflect.DeepEqual(got, want) {
		t.Fatalf("环境数组 = %#v, want %#v", got, want)
	}
}

func TestRedactEnvironment_whenSecretNamesProvided_shouldHideValues(t *testing.T) {
	environment := map[string]string{
		"PORT":              "9090",
		"DATABASE_PASSWORD": "database-secret",
		"api_key":           "api-secret",
		"ACCESS_TOKEN":      "access-secret",
	}
	redacted := RedactEnvironment(environment)
	if redacted["PORT"] != "9090" {
		t.Fatalf("普通环境变量被脱敏: %#v", redacted)
	}
	for _, name := range []string{"DATABASE_PASSWORD", "api_key", "ACCESS_TOKEN"} {
		if redacted[name] != RedactedValue {
			t.Fatalf("敏感变量 %s 未脱敏: %#v", name, redacted)
		}
	}
}
