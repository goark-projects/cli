package cli

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseRunArguments_whenGoarkAndApplicationArgumentsMixed_shouldClassifyWithoutReordering(t *testing.T) {
	plan, err := parseRunArguments([]string{
		"-race",
		"-tags=dev,integration",
		"-Dserver.port=9090",
		"./cmd/server",
		"--goark.profiles.active=dev",
		"--",
		"--job=sync",
		"input.json",
	})
	if err != nil {
		t.Fatalf("解析 run 参数失败: %v", err)
	}

	if !reflect.DeepEqual(plan.GoArguments, []string{"-race", "-tags=dev,integration", "./cmd/server"}) {
		t.Fatalf("GoArguments = %#v", plan.GoArguments)
	}
	if !reflect.DeepEqual(plan.PropertyArguments, []string{"-Dserver.port=9090", "--goark.profiles.active=dev"}) {
		t.Fatalf("PropertyArguments = %#v", plan.PropertyArguments)
	}
	if !reflect.DeepEqual(plan.ApplicationArguments, []string{"--job=sync", "input.json"}) {
		t.Fatalf("ApplicationArguments = %#v", plan.ApplicationArguments)
	}
	if plan.Target != "./cmd/server" || !plan.TargetExplicit {
		t.Fatalf("目标解析错误: %#v", plan)
	}
	if got := plan.GoRunArguments(); !reflect.DeepEqual(got, []string{
		"-race",
		"-tags=dev,integration",
		"./cmd/server",
		"-Dserver.port=9090",
		"--goark.profiles.active=dev",
		"--job=sync",
		"input.json",
	}) {
		t.Fatalf("GoRunArguments() = %#v", got)
	}
}

func TestParseRunArguments_whenBuildFlagConsumesValue_shouldKeepTargetBoundary(t *testing.T) {
	plan, err := parseRunArguments([]string{"-tags", "dev", "-ldflags", "-s -w", "./cmd/server", "arg"})
	if err != nil {
		t.Fatalf("解析 run 参数失败: %v", err)
	}
	if !reflect.DeepEqual(plan.GoArguments, []string{"-tags", "dev", "-ldflags", "-s -w", "./cmd/server"}) {
		t.Fatalf("GoArguments = %#v", plan.GoArguments)
	}
	if !reflect.DeepEqual(plan.ApplicationArguments, []string{"arg"}) {
		t.Fatalf("ApplicationArguments = %#v", plan.ApplicationArguments)
	}
}

func TestParseRunArguments_whenCurrentGoBuildFlagsConsumeValues_shouldKeepTargetBoundary(t *testing.T) {
	flags := []string{"-buildvcs", "true", "-covermode", "atomic", "-coverpkg", "./...", "-pgo", "auto"}
	plan, err := parseRunArguments(append(flags, "./cmd/server", "application-argument"))
	if err != nil {
		t.Fatalf("解析 run 参数失败: %v", err)
	}
	if !reflect.DeepEqual(plan.GoArguments, append(flags, "./cmd/server")) {
		t.Fatalf("GoArguments = %#v", plan.GoArguments)
	}
	if !reflect.DeepEqual(plan.ApplicationArguments, []string{"application-argument"}) {
		t.Fatalf("ApplicationArguments = %#v", plan.ApplicationArguments)
	}
}

func TestParseRunArguments_whenGoFilesProvided_shouldKeepAllGoFilesAsTargets(t *testing.T) {
	plan, err := parseRunArguments([]string{"main.go", "wire.go", "--", "value"})
	if err != nil {
		t.Fatalf("解析 run 参数失败: %v", err)
	}
	if !reflect.DeepEqual(plan.GoArguments, []string{"main.go", "wire.go"}) {
		t.Fatalf("GoArguments = %#v", plan.GoArguments)
	}
	if plan.Target != "main.go" || !plan.TargetExplicit {
		t.Fatalf("目标解析错误: %#v", plan)
	}
	if !reflect.DeepEqual(plan.ApplicationArguments, []string{"value"}) {
		t.Fatalf("ApplicationArguments = %#v", plan.ApplicationArguments)
	}
}

func TestComposeRunArguments_whenApplicationUsesGlobalFlagName_shouldNotMoveApplicationArgument(t *testing.T) {
	plan, err := parseRunArguments([]string{"./cmd/server", "--", "-C=application-value"})
	if err != nil {
		t.Fatalf("解析 run 参数失败: %v", err)
	}
	args := composeEnhancedGoArguments("run", plan.GoArguments)
	args = append(args, plan.PropertyArguments...)
	args = append(args, plan.ApplicationArguments...)
	want := []string{"run", "./cmd/server", "-C=application-value"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("最终参数 = %#v, want %#v", args, want)
	}
}

func TestParseRunArguments_whenNoTargetProvided_shouldKeepPropertiesForResolvedTarget(t *testing.T) {
	plan, err := parseRunArguments([]string{"-Dserver.port=9090", "--feature.enabled=true"})
	if err != nil {
		t.Fatalf("解析 run 参数失败: %v", err)
	}
	if plan.TargetExplicit || len(plan.GoArguments) != 0 {
		t.Fatalf("不应存在显式目标: %#v", plan)
	}
	if !reflect.DeepEqual(plan.PropertyArguments, []string{"-Dserver.port=9090", "--feature.enabled=true"}) {
		t.Fatalf("PropertyArguments = %#v", plan.PropertyArguments)
	}
}

func TestParseRunArguments_whenLongPropertyUsesSeparateValue_shouldNotTreatValueAsTarget(t *testing.T) {
	plan, err := parseRunArguments([]string{"--server.port", "9090", "--feature.enabled", "true"})
	if err != nil {
		t.Fatalf("解析 run 参数失败: %v", err)
	}
	if plan.TargetExplicit {
		t.Fatalf("属性值不应成为运行目标: %#v", plan)
	}
	if !reflect.DeepEqual(plan.PropertyArguments, []string{"--server.port", "9090", "--feature.enabled", "true"}) {
		t.Fatalf("PropertyArguments = %#v", plan.PropertyArguments)
	}
}

func TestParseRunArguments_whenControlFlagsProvided_shouldSetExecutionMode(t *testing.T) {
	plan, err := parseRunArguments([]string{"--goark-profile=dev", "--goark-env=PORT=9090", "--goark-offline", "--goark-locked", "--goark-dry-run", "."})
	if err != nil {
		t.Fatalf("解析 run 参数失败: %v", err)
	}
	if plan.Control.Profile != "dev" || !plan.Control.DryRun || !plan.Control.Offline || !plan.Control.Locked || plan.Control.Environment["PORT"] != "9090" {
		t.Fatalf("控制参数解析错误: %#v", plan)
	}
}

func TestParseRunArguments_whenRemovedControlFlagsProvided_shouldReject(t *testing.T) {
	for _, argument := range []string{"--goark-generate-only", "--goark-no-generate"} {
		if _, err := parseRunArguments([]string{argument}); err == nil {
			t.Fatalf("已删除参数必须失败: %s", argument)
		}
	}
}

func TestParseRunArguments_whenSystemPropertyInvalid_shouldReject(t *testing.T) {
	for _, input := range [][]string{{"-D"}, {"-Dserver.port"}, {"-D=9090"}} {
		if _, err := parseRunArguments(input); err == nil {
			t.Fatalf("非法系统属性应失败: %#v", input)
		}
	}
}

func TestEffectiveGoWorkingDir_whenDirectoryFlagRepeated_shouldUseLastValue(t *testing.T) {
	base := t.TempDir()
	workingDir, err := effectiveGoWorkingDir(base, []string{"-C", "first", "-C=second"})
	if err != nil {
		t.Fatalf("解析工作目录失败: %v", err)
	}
	want := filepath.Join(base, "second")
	if workingDir != want {
		t.Fatalf("工作目录 = %q, want %q", workingDir, want)
	}
}
