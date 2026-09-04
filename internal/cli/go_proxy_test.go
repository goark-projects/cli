package cli

import (
	"bytes"
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
	runner := &recordingProcessRunner{}
	command := Command{Out: io.Discard, Err: io.Discard, Runner: runner}

	if code := command.Run([]string{"build", "--goark-no-generate", "-C", "service", "./..."}); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	assertSingleGoRequest(t, runner, []string{"-C", "service", "build", "./..."})
}

func TestCommand_whenInstallingVersionedPackageOutsideProject_shouldSkipLocalGeneration(t *testing.T) {
	runner := &recordingProcessRunner{}
	command := Command{Out: io.Discard, Err: io.Discard, Runner: runner}

	if code := command.Run([]string{"install", "example.com/tool@latest"}); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	assertSingleGoRequest(t, runner, []string{"install", "example.com/tool@latest"})
}

func TestCommand_whenEnhancedTestApplicationUsesDirectoryFlag_shouldKeepItAfterArgsBoundary(t *testing.T) {
	runner := &recordingProcessRunner{}
	command := Command{Out: io.Discard, Err: io.Discard, Runner: runner}

	if code := command.Run([]string{"test", "--goark-no-generate", "./...", "-args", "-C", "test-value"}); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	assertSingleGoRequest(t, runner, []string{"test", "./...", "-args", "-C", "test-value"})
}

func TestCommand_whenEnhancedTestApplicationRequestsHelp_shouldPassThroughAfterArgsBoundary(t *testing.T) {
	runner := &recordingProcessRunner{}
	command := Command{Out: io.Discard, Err: io.Discard, Runner: runner}

	if code := command.Run([]string{"test", "--goark-no-generate", "./...", "-args", "--help"}); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	assertSingleGoRequest(t, runner, []string{"test", "./...", "-args", "--help"})
}

func TestCommand_whenGoProcessExitsNonzero_shouldReturnChildExitCode(t *testing.T) {
	runner := &recordingProcessRunner{err: processExitError{code: 37}}
	command := Command{Out: io.Discard, Err: io.Discard, Runner: runner}

	if code := command.Run([]string{"go", "test", "./..."}); code != 37 {
		t.Fatalf("退出码 = %d", code)
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
