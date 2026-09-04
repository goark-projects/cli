package taskgraph

import (
	"fmt"
	"sort"
	"strings"

	"goark.dev/cli/internal/buildspec"
)

// Graph 是经过完整静态校验的不可变任务图。
type Graph struct {
	tasks      map[string]buildspec.Task
	dependents map[string][]string
}

// New 构建任务图并在执行前完成结构与输出冲突校验。
func New(tasks map[string]buildspec.Task) (*Graph, error) {
	cloned := make(map[string]buildspec.Task, len(tasks))
	dependents := make(map[string][]string, len(tasks))
	for name, task := range tasks {
		task.DependsOn = append([]string(nil), task.DependsOn...)
		task.Inputs = append([]string(nil), task.Inputs...)
		task.Outputs = append([]string(nil), task.Outputs...)
		cloned[name] = task
		seen := make(map[string]struct{}, len(task.DependsOn))
		for _, dependency := range task.DependsOn {
			if _, exists := seen[dependency]; exists {
				return nil, fmt.Errorf("任务 %q 重复依赖 %q", name, dependency)
			}
			seen[dependency] = struct{}{}
			if _, exists := tasks[dependency]; !exists {
				return nil, fmt.Errorf("任务 %q 依赖不存在的任务 %q", name, dependency)
			}
			dependents[dependency] = append(dependents[dependency], name)
		}
	}
	for name := range dependents {
		sort.Strings(dependents[name])
	}
	graph := &Graph{tasks: cloned, dependents: dependents}
	if err := graph.validateCycles(); err != nil {
		return nil, err
	}
	if err := validateOutputConflicts(cloned); err != nil {
		return nil, err
	}
	return graph, nil
}

// Order 返回目标任务及全部上游依赖的确定性拓扑顺序。
func (g *Graph) Order(targets []string) ([]string, error) {
	visited := make(map[string]bool, len(g.tasks))
	order := make([]string, 0, len(g.tasks))
	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		task, exists := g.tasks[name]
		if !exists {
			return fmt.Errorf("目标任务 %q 不存在", name)
		}
		for _, dependency := range task.DependsOn {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		visited[name] = true
		order = append(order, name)
		return nil
	}
	for _, target := range targets {
		if err := visit(target); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// Dependents 返回指定任务的直接下游任务。
func (g *Graph) Dependents(name string) []string {
	return append([]string(nil), g.dependents[name]...)
}

// Task 返回任务定义的副本。
func (g *Graph) Task(name string) (buildspec.Task, bool) {
	task, ok := g.tasks[name]
	return task, ok
}

func (g *Graph) validateCycles() error {
	const (
		unvisited = iota
		visiting
		visited
	)
	states := make(map[string]int, len(g.tasks))
	stack := make([]string, 0, len(g.tasks))
	var visit func(string) error
	visit = func(name string) error {
		switch states[name] {
		case visiting:
			start := 0
			for index, item := range stack {
				if item == name {
					start = index
					break
				}
			}
			cycle := append(append([]string(nil), stack[start:]...), name)
			return fmt.Errorf("任务图存在循环依赖: %s", strings.Join(cycle, " -> "))
		case visited:
			return nil
		}
		states[name] = visiting
		stack = append(stack, name)
		for _, dependency := range g.tasks[name].DependsOn {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		states[name] = visited
		return nil
	}
	for _, name := range sortedTaskNames(g.tasks) {
		if states[name] == unvisited {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

func sortedTaskNames(tasks map[string]buildspec.Task) []string {
	names := make([]string, 0, len(tasks))
	for name := range tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
