package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

type processExitError struct {
	code int
}

func (e processExitError) Error() string {
	return "进程退出"
}

func (e processExitError) ExitCode() int {
	return e.code
}

type recordingProcessRunner struct {
	requests []ProcessRequest
	err      error
}

func (r *recordingProcessRunner) Run(request ProcessRequest) error {
	r.requests = append(r.requests, request)
	return r.err
}

func TestCommand_whenGoVersionRequested_shouldDelegateToGo(t *testing.T) {
	runner := &recordingProcessRunner{}
	command := Command{Out: io.Discard, Err: io.Discard, Runner: runner}

	if code := command.Run([]string{"go", "version"}); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	assertSingleGoRequest(t, runner, []string{"version"})
}

func TestCommand_whenGoHasNoArguments_shouldStillDelegateExactly(t *testing.T) {
	runner := &recordingProcessRunner{}
	command := Command{Out: io.Discard, Err: io.Discard, Runner: runner}

	if code := command.Run([]string{"go"}); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	assertSingleGoRequest(t, runner, nil)
}

func TestCommand_whenGoGenerateRequested_shouldDelegateWithoutLegacyDispatch(t *testing.T) {
	runner := &recordingProcessRunner{}
	command := Command{Out: io.Discard, Err: io.Discard, Runner: runner}

	if code := command.Run([]string{"go", "generate", "./..."}); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	assertSingleGoRequest(t, runner, []string{"generate", "./..."})
}

func TestCommand_whenCLIVersionRequested_shouldNotStartGo(t *testing.T) {
	var stdout bytes.Buffer
	runner := &recordingProcessRunner{}
	command := Command{Out: &stdout, Err: io.Discard, Runner: runner}

	if code := command.Run([]string{"--version"}); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	if stdout.String() != "goark "+Version+"\n" {
		t.Fatalf("版本输出 = %q", stdout.String())
	}
	if len(runner.requests) != 0 {
		t.Fatalf("不应启动 go: %#v", runner.requests)
	}
}

func TestCommand_whenGlobalGoFlagsProvided_shouldDelegateInOriginalOrder(t *testing.T) {
	runner := &recordingProcessRunner{}
	command := Command{Out: io.Discard, Err: io.Discard, Runner: runner}
	args := []string{"-C", "service", "env", "GOMOD"}

	if code := command.Run(append([]string{"go"}, args...)); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	assertSingleGoRequest(t, runner, args)
}

func TestCommand_whenEnvironmentProvided_shouldDelegateWithoutModification(t *testing.T) {
	runner := &recordingProcessRunner{}
	environment := []string{"GOOS=linux", "GOARCH=arm64", "GOWORK=off", "GOTOOLCHAIN=local", "CUSTOM_VALUE=original"}
	command := Command{Out: io.Discard, Err: io.Discard, Env: environment, Runner: runner}

	if code := command.Run([]string{"go", "env", "GOOS"}); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	assertSingleGoRequest(t, runner, []string{"env", "GOOS"})
	if !reflect.DeepEqual(runner.requests[0].Env, environment) {
		t.Fatalf("环境变量 = %#v, want %#v", runner.requests[0].Env, environment)
	}
}

func TestCommand_whenEnhancedBuildUsesGlobalDirectoryFlag_shouldPlaceFlagBeforeGoCommand(t *testing.T) {
	got := composeEnhancedGoArguments("build", []string{"-C", "service", "./..."})
	want := []string{"-C", "service", "build", "./..."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("参数 = %#v, want %#v", got, want)
	}
}

func TestCommand_whenInstallingVersionedPackageOutsideProject_shouldRequireBuildFile(t *testing.T) {
	var stderr bytes.Buffer
	runner := &recordingProcessRunner{}
	command := Command{Out: io.Discard, Err: &stderr, Runner: runner}

	if code := command.Run([]string{"install", "example.com/tool@latest"}); code != 2 {
		t.Fatalf("退出码 = %d", code)
	}
	if len(runner.requests) != 1 || runner.requests[0].Args[0] != "list" || !strings.Contains(stderr.String(), "本地 Go 模块") {
		t.Fatalf("请求 = %#v, stderr=%q", runner.requests, stderr.String())
	}
}

func TestCommand_whenEnhancedTestApplicationUsesDirectoryFlag_shouldKeepItAfterArgsBoundary(t *testing.T) {
	remaining, _, err := parseWorkflowArguments([]string{"./...", "-args", "-C", "test-value"})
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}
	got := composeEnhancedGoArguments("test", remaining)
	want := []string{"test", "./...", "-args", "-C", "test-value"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("参数 = %#v, want %#v", got, want)
	}
}

func TestCommand_whenEnhancedTestApplicationRequestsHelp_shouldPassThroughAfterArgsBoundary(t *testing.T) {
	remaining, _, err := parseWorkflowArguments([]string{"./...", "-args", "--help"})
	if err != nil {
		t.Fatalf("解析参数失败: %v", err)
	}
	got := composeEnhancedGoArguments("test", remaining)
	want := []string{"test", "./...", "-args", "--help"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("参数 = %#v, want %#v", got, want)
	}
}

func TestCommand_whenGoProcessExitsNonzero_shouldReturnChildExitCode(t *testing.T) {
	runner := &recordingProcessRunner{err: processExitError{code: 37}}
	command := Command{Out: io.Discard, Err: io.Discard, Runner: runner}

	if code := command.Run([]string{"go", "test", "./..."}); code != 37 {
		t.Fatalf("退出码 = %d", code)
	}
}

func TestCommand_whenGoProcessCanceled_shouldReturnInterruptExitCode(t *testing.T) {
	var stderr bytes.Buffer
	runner := &recordingProcessRunner{err: context.Canceled}
	command := Command{Out: io.Discard, Err: &stderr, Runner: runner}

	if code := command.Run([]string{"go", "test", "./..."}); code != 130 {
		t.Fatalf("退出码 = %d", code)
	}
	if strings.Contains(stderr.String(), "启动 go 失败") {
		t.Fatalf("取消不应报告启动失败: %q", stderr.String())
	}
}

func TestCommand_whenGoProcessCannotStart_shouldReturnOneAndReportCause(t *testing.T) {
	var stderr bytes.Buffer
	runner := &recordingProcessRunner{err: errors.New("executable file not found")}
	command := Command{Out: io.Discard, Err: &stderr, Runner: runner}

	if code := command.Run([]string{"go", "version"}); code != 1 {
		t.Fatalf("退出码 = %d", code)
	}
	if !strings.Contains(stderr.String(), "启动 go 失败") || !strings.Contains(stderr.String(), "executable file not found") {
		t.Fatalf("启动错误不完整: %q", stderr.String())
	}
}

func assertSingleGoRequest(t *testing.T, runner *recordingProcessRunner, args []string) {
	t.Helper()
	if len(runner.requests) != 1 {
		t.Fatalf("进程请求数量 = %d", len(runner.requests))
	}
	request := runner.requests[0]
	if request.Name != "go" || !reflect.DeepEqual(request.Args, args) {
		t.Fatalf("进程请求 = %#v", request)
	}
}
