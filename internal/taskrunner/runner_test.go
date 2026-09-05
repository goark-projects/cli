package taskrunner

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/processrun"
	"goark.dev/cli/internal/taskcache"
	"goark.dev/cli/internal/tooling"
)

type recordingRunner struct {
	requests []processrun.Request
	err      error
}

type blockingRunner struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (r *blockingRunner) Run(processrun.Request) error {
	r.calls.Add(1)
	r.started <- struct{}{}
	<-r.release
	return nil
}

func (r *recordingRunner) Run(request processrun.Request) error {
	r.requests = append(r.requests, request)
	return r.err
}

func TestRunnerRun_whenExecTaskProvided_shouldExpandAndExecute(t *testing.T) {
	root := t.TempDir()
	process := &recordingRunner{}
	var output bytes.Buffer
	runner := New(Options{
		Root: root, ProjectName: "demo", ProjectModule: "example.com/demo", Profile: "dev",
		CommandOutput: "build/demo", Environment: map[string]string{"CONFIG": "resource/config.toml"},
		Tools:   map[string]tooling.Resolved{"tool": {Name: "tool", Path: filepath.Join(root, "tool")}},
		Process: process, Out: &output, Err: &output, DefaultTimeout: time.Minute,
	})
	task := buildspec.Task{
		Type: buildspec.TaskTypeExec, Tool: "tool",
		Args:             []string{"--root", "${project.root}", "${env:CONFIG}", "${command.output}"},
		Environment:      map[string]string{"TASK_NAME": "${project.name}"},
		WorkingDirectory: ".",
		When:             `profile == "dev"`,
	}
	if err := runner.Run(context.Background(), "exec", task); err != nil {
		t.Fatalf("执行任务失败: %v", err)
	}
	if len(process.requests) != 1 {
		t.Fatalf("进程数量 = %d", len(process.requests))
	}
	request := process.requests[0]
	wantArgs := []string{"--root", root, "resource/config.toml", "build/demo"}
	if request.Name != filepath.Join(root, "tool") || request.Dir != root || !reflect.DeepEqual(request.Args, wantArgs) {
		t.Fatalf("进程请求 = %#v", request)
	}
	if !containsEnvironment(request.Env, "TASK_NAME=demo") {
		t.Fatalf("任务环境缺失: %#v", request.Env)
	}
}

func TestRunnerRun_whenWindowsEnvironmentNamesDifferOnlyByCase_shouldPreserveOverridePrecedence(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅适用于 Windows 环境名语义")
	}
	process := &recordingRunner{}
	runner := New(Options{
		Root: t.TempDir(), Environment: map[string]string{"Path": "process"},
		OverrideEnvironment: map[string]string{"path": "cli"}, Process: process,
	})
	task := buildspec.Task{Type: buildspec.TaskTypeGo, Args: []string{"version"}, Environment: map[string]string{"PATH": "task"}}
	if err := runner.Run(context.Background(), "environment", task); err != nil {
		t.Fatalf("执行任务失败: %v", err)
	}
	if len(process.requests) != 1 {
		t.Fatalf("进程数量 = %d", len(process.requests))
	}
	var matches []string
	for _, entry := range process.requests[0].Env {
		name, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(name, "PATH") {
			matches = append(matches, entry)
		}
	}
	if !reflect.DeepEqual(matches, []string{"path=cli"}) {
		t.Fatalf("PATH 环境 = %#v", matches)
	}
}

func TestRunnerRun_whenWorkingDirectoryUsesEnvironmentVariable_shouldValidateExpandedPath(t *testing.T) {
	root := t.TempDir()
	workingDirectory := filepath.Join(root, "cmd")
	if err := os.Mkdir(workingDirectory, 0o755); err != nil {
		t.Fatalf("创建工作目录失败: %v", err)
	}
	process := &recordingRunner{}
	runner := New(Options{Root: root, Environment: map[string]string{"WORKDIR": "cmd"}, Process: process})
	task := buildspec.Task{Type: buildspec.TaskTypeGo, Args: []string{"version"}, WorkingDirectory: "${env:WORKDIR}"}
	if err := runner.Run(context.Background(), "working-directory", task); err != nil {
		t.Fatalf("执行任务失败: %v", err)
	}
	if len(process.requests) != 1 || process.requests[0].Dir != workingDirectory {
		t.Fatalf("工作目录 = %#v", process.requests)
	}
}

func TestRunnerRun_whenConditionIsFalse_shouldSkipProcess(t *testing.T) {
	process := &recordingRunner{}
	runner := New(Options{Root: t.TempDir(), Profile: "dev", Process: process})
	err := runner.Run(context.Background(), "skip", buildspec.Task{Type: buildspec.TaskTypeGo, Args: []string{"version"}, When: `profile == "production"`})
	if err != nil {
		t.Fatalf("跳过任务失败: %v", err)
	}
	if len(process.requests) != 0 {
		t.Fatalf("不应启动进程: %#v", process.requests)
	}
}

func TestRunnerRun_whenDeleteTaskEscapesProject_shouldRejectWithoutDeleting(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	outside := filepath.Join(parent, "outside.txt")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("创建项目目录失败: %v", err)
	}
	if err := os.WriteFile(outside, []byte("keep"), 0o644); err != nil {
		t.Fatalf("写入外部文件失败: %v", err)
	}
	runner := New(Options{Root: root, Process: &recordingRunner{}})
	err := runner.Run(context.Background(), "delete", buildspec.Task{Type: buildspec.TaskTypeDelete, Outputs: []string{"../outside.txt"}})
	if err == nil || !strings.Contains(err.Error(), "项目根目录") {
		t.Fatalf("错误 = %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("外部文件被删除: %v", err)
	}
}

func TestRunnerRun_whenCacheHits_shouldNotStartProcess(t *testing.T) {
	root := t.TempDir()
	writeTaskFile(t, root, "input/source.txt", "input")
	writeTaskFile(t, root, "output/result.txt", "result")
	task := buildspec.Task{Type: buildspec.TaskTypeGo, Args: []string{"version"}, Inputs: []string{"input/*.txt"}, Outputs: []string{"output/*.txt"}, Cache: true}
	cache := taskcache.NewStore(root)
	if err := cache.Save(taskcache.Context{Root: root, TaskName: "cached", Task: task, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}); err != nil {
		t.Fatalf("准备缓存失败: %v", err)
	}
	process := &recordingRunner{}
	runner := New(Options{Root: root, Process: process, Cache: cache})
	if err := runner.Run(context.Background(), "cached", task); err != nil {
		t.Fatalf("读取缓存失败: %v", err)
	}
	if len(process.requests) != 0 {
		t.Fatalf("缓存命中后不应启动进程: %#v", process.requests)
	}
}

func TestRunnerRun_whenTaskTimesOut_shouldReturnDeadline(t *testing.T) {
	process := &recordingRunner{err: context.DeadlineExceeded}
	runner := New(Options{Root: t.TempDir(), Process: process, DefaultTimeout: time.Nanosecond})
	err := runner.Run(context.Background(), "timeout", buildspec.Task{Type: buildspec.TaskTypeGo, Args: []string{"version"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("错误 = %v", err)
	}
}

func TestRunnerRun_whenDryRunRequested_shouldNotStartProcessOrDelete(t *testing.T) {
	root := t.TempDir()
	writeTaskFile(t, root, "output/result.txt", "keep")
	process := &recordingRunner{}
	var diagnostic bytes.Buffer
	runner := New(Options{Root: root, Process: process, DryRun: true, Err: &diagnostic})
	if err := runner.Run(context.Background(), "delete", buildspec.Task{Type: buildspec.TaskTypeDelete, Outputs: []string{"output/result.txt"}}); err != nil {
		t.Fatalf("模拟删除失败: %v", err)
	}
	if len(process.requests) != 0 {
		t.Fatalf("模拟执行不应启动进程: %#v", process.requests)
	}
	if _, err := os.Stat(filepath.Join(root, "output", "result.txt")); err != nil {
		t.Fatalf("模拟执行删除了文件: %v", err)
	}
	if !strings.Contains(diagnostic.String(), "would run task delete") {
		t.Fatalf("模拟执行诊断缺失: %q", diagnostic.String())
	}
}

func TestRunnerRun_whenSameTaskStartsConcurrently_shouldExecuteOnlyOnce(t *testing.T) {
	process := &blockingRunner{started: make(chan struct{}, 2), release: make(chan struct{})}
	runner := New(Options{Root: t.TempDir(), Process: process})
	task := buildspec.Task{Type: buildspec.TaskTypeGo, Args: []string{"version"}}
	start := make(chan struct{})
	errors := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			errors <- runner.Run(context.Background(), "once", task)
		}()
	}
	ready.Wait()
	close(start)
	<-process.started
	select {
	case <-process.started:
		close(process.release)
		t.Fatal("同一任务被并发执行了多次")
	case <-time.After(100 * time.Millisecond):
		close(process.release)
	}
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatalf("执行任务失败: %v", err)
		}
	}
	if process.calls.Load() != 1 {
		t.Fatalf("进程执行次数 = %d", process.calls.Load())
	}
}

func containsEnvironment(environment []string, expected string) bool {
	for _, value := range environment {
		if value == expected {
			return true
		}
	}
	return false
}

func writeTaskFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
}
