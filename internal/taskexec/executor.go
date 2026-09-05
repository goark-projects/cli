package taskexec

import (
	"context"
	"fmt"

	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/taskgraph"
)

// Runner 执行单个已经满足依赖的任务。
type Runner interface {
	Run(context.Context, string, buildspec.Task) error
}

// RunnerFunc 将函数适配为任务运行器。
type RunnerFunc func(context.Context, string, buildspec.Task) error

// Run 执行任务函数。
func (f RunnerFunc) Run(ctx context.Context, name string, task buildspec.Task) error {
	return f(ctx, name, task)
}

// TaskError 保留失败任务名称与底层错误。
type TaskError struct {
	Name string
	Err  error
}

func (e TaskError) Error() string {
	return fmt.Sprintf("任务 %q 执行失败: %v", e.Name, e.Err)
}

func (e TaskError) Unwrap() error {
	return e.Err
}

// Executor 按依赖关系和并发声明执行任务闭包。
type Executor struct {
	Graph       *taskgraph.Graph
	MaxParallel int
	FailFast    bool
	Runner      Runner
}

// Execute 执行目标任务及其全部依赖。
func (e Executor) Execute(ctx context.Context, targets []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e.Graph == nil {
		return fmt.Errorf("任务图不能为空")
	}
	if e.Runner == nil {
		return fmt.Errorf("任务运行器不能为空")
	}
	order, err := e.Graph.Order(targets)
	if err != nil {
		return err
	}
	limit := e.MaxParallel
	if limit < 1 {
		limit = 1
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	states := make(map[string]taskState, len(order))
	var firstFailure error
	remaining := len(order)
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		ready := make([]string, 0, limit)
		progressed := false
		for _, name := range order {
			if states[name] != statePending {
				continue
			}
			task, _ := e.Graph.Task(name)
			readyNow, blocked := dependenciesReady(task, states)
			if blocked {
				states[name] = stateSkipped
				remaining--
				progressed = true
				if firstFailure == nil {
					firstFailure = TaskError{Name: name, Err: fmt.Errorf("上游任务失败")}
				}
				continue
			}
			if readyNow {
				ready = append(ready, name)
			}
		}
		if len(ready) == 0 {
			if progressed {
				continue
			}
			break
		}

		batch := selectBatch(e.Graph, ready, limit)
		failures, batchFailure := e.runBatch(runCtx, cancel, batch)
		if e.FailFast && batchFailure.err != nil && firstFailure == nil {
			firstFailure = TaskError{Name: batchFailure.name, Err: batchFailure.err}
		}
		for _, name := range batch {
			remaining--
			if taskErr := failures[name]; taskErr != nil {
				states[name] = stateFailed
				if firstFailure == nil {
					firstFailure = TaskError{Name: name, Err: taskErr}
				}
				continue
			}
			states[name] = stateSucceeded
		}
		if e.FailFast && firstFailure != nil {
			return firstFailure
		}
	}
	if firstFailure != nil {
		return firstFailure
	}
	if remaining != 0 {
		return fmt.Errorf("任务图调度停止，仍有 %d 个任务未处理", remaining)
	}
	return nil
}

type taskState uint8

const (
	statePending taskState = iota
	stateSucceeded
	stateFailed
	stateSkipped
)

func dependenciesReady(task buildspec.Task, states map[string]taskState) (bool, bool) {
	for _, dependency := range task.DependsOn {
		switch states[dependency] {
		case stateFailed, stateSkipped:
			return false, true
		case stateSucceeded:
		default:
			return false, false
		}
	}
	return true, false
}

func selectBatch(graph *taskgraph.Graph, ready []string, limit int) []string {
	for _, name := range ready {
		task, _ := graph.Task(name)
		if !task.ParallelSafe {
			return []string{name}
		}
	}
	if len(ready) > limit {
		ready = ready[:limit]
	}
	return ready
}

type taskResult struct {
	name string
	err  error
}

func (e Executor) runBatch(ctx context.Context, cancel context.CancelFunc, names []string) (map[string]error, taskResult) {
	results := make(chan taskResult, len(names))
	for _, name := range names {
		task, _ := e.Graph.Task(name)
		go func(taskName string, taskDefinition buildspec.Task) {
			results <- taskResult{name: taskName, err: e.Runner.Run(ctx, taskName, taskDefinition)}
		}(name, task)
	}
	failures := make(map[string]error)
	var firstFailure taskResult
	for range names {
		result := <-results
		if result.err != nil {
			failures[result.name] = result.err
			if e.FailFast && firstFailure.err == nil {
				firstFailure = result
				cancel()
			}
		}
	}
	return failures, firstFailure
}
