package taskrunner

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"goark.dev/cli/internal/buildplan"
	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/condition"
	"goark.dev/cli/internal/processrun"
	"goark.dev/cli/internal/taskcache"
	"goark.dev/cli/internal/tooling"
)

// Options 描述任务运行器的项目级依赖。
type Options struct {
	Root                string
	ProjectName         string
	ProjectModule       string
	Profile             string
	CommandOutput       string
	Environment         map[string]string
	OverrideEnvironment map[string]string
	Tools               map[string]tooling.Resolved
	Process             processrun.Runner
	Cache               taskcache.Store
	In                  io.Reader
	Out                 io.Writer
	Err                 io.Writer
	DefaultTimeout      time.Duration
	DryRun              bool
	GoVersion           string
	GOOS                string
	GOARCH              string
	BuildTags           []string
}

// Runner 将任务定义适配到进程、文件系统和缓存边界。
type Runner struct {
	options   Options
	mu        sync.Mutex
	upstream  map[string]string
	completed map[string]bool
	running   map[string]*taskExecution
}

type taskExecution struct {
	done chan struct{}
	err  error
}

// New 创建任务运行器。
func New(options Options) *Runner {
	if options.Process == nil {
		options.Process = processrun.OSRunner{}
	}
	if options.Out == nil {
		options.Out = io.Discard
	}
	if options.Err == nil {
		options.Err = io.Discard
	}
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}
	if options.GOARCH == "" {
		options.GOARCH = runtime.GOARCH
	}
	return &Runner{
		options: options, upstream: make(map[string]string),
		completed: make(map[string]bool), running: make(map[string]*taskExecution),
	}
}

// Run 执行单个任务并更新供下游指纹使用的输出摘要。
func (r *Runner) Run(ctx context.Context, name string, task buildspec.Task) error {
	execution, leader := r.begin(name)
	if !leader {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-execution.done:
			return execution.err
		}
	}
	err := r.run(ctx, name, task)
	r.finish(name, execution, err)
	return err
}

func (r *Runner) begin(name string) (*taskExecution, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.completed[name] {
		execution := &taskExecution{done: make(chan struct{})}
		close(execution.done)
		return execution, false
	}
	if execution, ok := r.running[name]; ok {
		return execution, false
	}
	execution := &taskExecution{done: make(chan struct{})}
	r.running[name] = execution
	return execution, true
}

func (r *Runner) finish(name string, execution *taskExecution, err error) {
	r.mu.Lock()
	execution.err = err
	if err == nil {
		r.completed[name] = true
	}
	delete(r.running, name)
	close(execution.done)
	r.mu.Unlock()
}

// ResetCompletion 允许新的生命周期阶段重新执行已经成功完成的任务。
func (r *Runner) ResetCompletion(names []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, name := range names {
		if _, ok := r.running[name]; ok {
			return fmt.Errorf("任务 %q 仍在运行，不能重置完成状态", name)
		}
	}
	for _, name := range names {
		delete(r.completed, name)
	}
	return nil
}

func (r *Runner) run(ctx context.Context, name string, task buildspec.Task) error {
	environment, values, err := r.taskValues(task)
	if err != nil {
		return err
	}
	enabled, err := condition.Evaluate(task.When, condition.Values{
		Profile: r.options.Profile, GOOS: r.options.GOOS, GOARCH: r.options.GOARCH, Environment: environment,
	})
	if err != nil {
		return err
	}
	if !enabled {
		r.setUpstream(name, "skipped")
		return nil
	}
	task, err = expandTask(task, values)
	if err != nil {
		return err
	}
	args := task.Args
	workingDirectory, err := r.workingDirectory(task.WorkingDirectory)
	if err != nil {
		return err
	}
	cacheContext := r.cacheContext(name, task, environment)
	if task.Cache && !r.options.DryRun {
		hit, err := r.options.Cache.Lookup(cacheContext)
		if err != nil {
			return err
		}
		if hit {
			digest, err := taskcache.OutputDigest(cacheContext)
			if err != nil {
				return err
			}
			r.setUpstream(name, digest)
			r.log("cache hit: %s\n", name)
			return nil
		}
	}
	if r.options.DryRun {
		r.printDryRun(name, task, args, environment)
		return nil
	}
	if err := r.execute(ctx, task, args, workingDirectory, environment); err != nil {
		return err
	}
	if task.Cache {
		if err := r.options.Cache.Save(cacheContext); err != nil {
			return err
		}
	}
	if err := r.recordOutput(name, task, cacheContext); err != nil {
		return err
	}
	return nil
}

func (r *Runner) execute(ctx context.Context, task buildspec.Task, args []string, workingDirectory string, environment map[string]string) error {
	timeout := task.Timeout.Duration
	if timeout == 0 {
		timeout = r.options.DefaultTimeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	switch task.Type {
	case buildspec.TaskTypeGroup:
		return nil
	case buildspec.TaskTypeDelete:
		return deleteOutputs(r.options.Root, task.Outputs)
	case buildspec.TaskTypeGo:
		return r.runProcess(ctx, "go", args, workingDirectory, environment)
	case buildspec.TaskTypeExec:
		tool, ok := r.options.Tools[task.Tool]
		if !ok {
			return fmt.Errorf("工具 %q 尚未解析", task.Tool)
		}
		return r.runProcess(ctx, tool.Path, args, workingDirectory, environment)
	default:
		return fmt.Errorf("不支持任务类型 %q", task.Type)
	}
}

func (r *Runner) runProcess(ctx context.Context, executable string, args []string, directory string, environment map[string]string) error {
	return r.options.Process.Run(processrun.Request{
		Context: ctx, Name: executable, Args: args, Dir: directory, Env: environmentList(environment),
		In: r.options.In, Out: r.options.Out, Err: r.options.Err,
	})
}

func (r *Runner) recordOutput(name string, task buildspec.Task, cacheContext taskcache.Context) error {
	if task.Type == buildspec.TaskTypeDelete {
		r.setUpstream(name, "deleted")
		return nil
	}
	if len(task.Outputs) == 0 {
		r.setUpstream(name, "completed")
		return nil
	}
	digest, err := taskcache.OutputDigest(cacheContext)
	if err != nil {
		return err
	}
	r.setUpstream(name, digest)
	return nil
}

func (r *Runner) printDryRun(name string, task buildspec.Task, args []string, environment map[string]string) {
	executable := string(task.Type)
	if task.Type == buildspec.TaskTypeExec {
		if tool, ok := r.options.Tools[task.Tool]; ok {
			executable = tool.Path
		}
	} else if task.Type == buildspec.TaskTypeGo {
		executable = "go"
	}
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, strconv.Quote(redact(executable, environment)))
	for _, argument := range args {
		parts = append(parts, strconv.Quote(redact(argument, environment)))
	}
	r.log("would run task %s: %s\n", name, strings.Join(parts, " "))
}

func (r *Runner) log(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.options.Err, format, args...)
}

func environmentList(environment map[string]string) []string {
	names := make([]string, 0, len(environment))
	for name := range environment {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, name+"="+environment[name])
	}
	return result
}

func redact(value string, environment map[string]string) string {
	redacted := buildplan.RedactEnvironment(environment)
	for name, replacement := range redacted {
		if replacement == buildplan.RedactedValue && environment[name] != "" {
			value = strings.ReplaceAll(value, environment[name], replacement)
		}
	}
	return value
}
