package taskexec

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/taskgraph"
)

func TestExecutor_whenParallelTasksAreReady_shouldRespectLimit(t *testing.T) {
	graph := mustGraph(t, map[string]buildspec.Task{
		"one":   {ParallelSafe: true},
		"two":   {ParallelSafe: true},
		"three": {ParallelSafe: true},
		"all":   {DependsOn: []string{"one", "two", "three"}},
	})
	release := make(chan struct{})
	started := make(chan struct{}, 3)
	var running atomic.Int32
	var maximum atomic.Int32
	runner := RunnerFunc(func(ctx context.Context, name string, _ buildspec.Task) error {
		if name == "all" {
			return nil
		}
		current := running.Add(1)
		defer running.Add(-1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			return nil
		}
	})
	executor := Executor{Graph: graph, MaxParallel: 2, FailFast: true, Runner: runner}
	done := make(chan error, 1)
	go func() { done <- executor.Execute(context.Background(), []string{"all"}) }()
	<-started
	<-started
	if maximum.Load() != 2 {
		t.Fatalf("最大并发 = %d", maximum.Load())
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("执行任务图失败: %v", err)
	}
}

func TestExecutor_whenTaskIsNotParallelSafe_shouldRunExclusively(t *testing.T) {
	graph := mustGraph(t, map[string]buildspec.Task{
		"parallel":  {ParallelSafe: true},
		"trigger":   {ParallelSafe: true},
		"exclusive": {ParallelSafe: false, DependsOn: []string{"trigger"}},
	})
	parallelStarted := make(chan struct{})
	releaseParallel := make(chan struct{})
	parallelDone := make(chan struct{})
	overlap := errors.New("独占任务与并行任务重叠")
	runner := RunnerFunc(func(_ context.Context, name string, _ buildspec.Task) error {
		switch name {
		case "parallel":
			close(parallelStarted)
			<-releaseParallel
			close(parallelDone)
		case "trigger":
			<-parallelStarted
		case "exclusive":
			select {
			case <-parallelDone:
			default:
				return overlap
			}
		}
		return nil
	})
	executor := Executor{Graph: graph, MaxParallel: 4, Runner: runner}
	done := make(chan error, 1)
	go func() { done <- executor.Execute(context.Background(), []string{"parallel", "exclusive"}) }()
	<-parallelStarted
	close(releaseParallel)
	if err := <-done; err != nil {
		t.Fatalf("执行任务图失败: %v", err)
	}
}

func TestExecutor_whenFailFastTaskFails_shouldCancelRunningTasksAndSkipPending(t *testing.T) {
	failure := errors.New("expected failure")
	graph := mustGraph(t, map[string]buildspec.Task{
		"fail":       {ParallelSafe: true},
		"wait":       {ParallelSafe: true},
		"downstream": {DependsOn: []string{"fail"}},
	})
	waitStarted := make(chan struct{})
	waitCanceled := make(chan struct{})
	var downstream atomic.Bool
	runner := RunnerFunc(func(ctx context.Context, name string, _ buildspec.Task) error {
		switch name {
		case "fail":
			<-waitStarted
			return failure
		case "wait":
			close(waitStarted)
			<-ctx.Done()
			close(waitCanceled)
			return ctx.Err()
		case "downstream":
			downstream.Store(true)
		}
		return nil
	})
	executor := Executor{Graph: graph, MaxParallel: 2, FailFast: true, Runner: runner}
	err := executor.Execute(context.Background(), []string{"fail", "wait", "downstream"})
	if !errors.Is(err, failure) {
		t.Fatalf("错误 = %v", err)
	}
	<-waitCanceled
	if downstream.Load() {
		t.Fatal("失败节点的下游任务不应执行")
	}
}

func TestExecutor_whenContextCanceledBeforeStart_shouldReturnCause(t *testing.T) {
	graph := mustGraph(t, map[string]buildspec.Task{"one": {}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executor := Executor{Graph: graph, MaxParallel: 1, Runner: RunnerFunc(func(context.Context, string, buildspec.Task) error {
		t.Fatal("取消后不应启动任务")
		return nil
	})}
	if err := executor.Execute(ctx, []string{"one"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("错误 = %v", err)
	}
}

func mustGraph(t *testing.T, tasks map[string]buildspec.Task) *taskgraph.Graph {
	t.Helper()
	graph, err := taskgraph.New(tasks)
	if err != nil {
		t.Fatalf("创建任务图失败: %v", err)
	}
	return graph
}
