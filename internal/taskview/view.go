// Package taskview 提供任务列表和依赖图的稳定只读表示。
package taskview

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"goark.dev/cli/internal/buildspec"
)

// Format 表示任务图输出格式。
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
	FormatDOT  Format = "dot"
)

// Task 是供终端和机器消费的稳定任务元数据。
type Task struct {
	Name         string             `json:"name"`
	Type         buildspec.TaskType `json:"type"`
	DependsOn    []string           `json:"dependsOn"`
	Inputs       []string           `json:"inputs,omitempty"`
	Outputs      []string           `json:"outputs,omitempty"`
	Cache        bool               `json:"cache"`
	ParallelSafe bool               `json:"parallelSafe"`
}

// Snapshot 按名称排序并复制任务元数据。
func Snapshot(tasks map[string]buildspec.Task) []Task {
	names := make([]string, 0, len(tasks))
	for name := range tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]Task, 0, len(names))
	for _, name := range names {
		task := tasks[name]
		result = append(result, Task{
			Name: name, Type: task.Type, DependsOn: append([]string(nil), task.DependsOn...),
			Inputs: append([]string(nil), task.Inputs...), Outputs: append([]string(nil), task.Outputs...),
			Cache: task.Cache, ParallelSafe: task.ParallelSafe,
		})
	}
	return result
}

// WriteList 输出任务列表。
func WriteList(writer io.Writer, tasks []Task, jsonOutput bool) error {
	if jsonOutput {
		return writeJSON(writer, tasks)
	}
	for _, task := range tasks {
		if _, err := fmt.Fprintf(writer, "%s\t%s\n", task.Name, task.Type); err != nil {
			return err
		}
	}
	return nil
}

// WriteGraph 输出确定性的任务依赖图。
func WriteGraph(writer io.Writer, tasks []Task, format Format) error {
	switch format {
	case FormatText:
		for _, task := range tasks {
			if _, err := io.WriteString(writer, textLine(task)); err != nil {
				return err
			}
		}
		return nil
	case FormatJSON:
		return writeJSON(writer, tasks)
	case FormatDOT:
		return writeDOT(writer, tasks)
	default:
		return fmt.Errorf("任务图格式 %q 无效", format)
	}
}

func textLine(task Task) string {
	if len(task.DependsOn) == 0 {
		return task.Name + "\n"
	}
	return task.Name + " -> " + strings.Join(task.DependsOn, ", ") + "\n"
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeDOT(writer io.Writer, tasks []Task) error {
	if _, err := io.WriteString(writer, "digraph goark {\n"); err != nil {
		return err
	}
	for _, task := range tasks {
		name := strconv.Quote(task.Name)
		if len(task.DependsOn) == 0 {
			if _, err := fmt.Fprintf(writer, "  %s;\n", name); err != nil {
				return err
			}
			continue
		}
		for _, dependency := range task.DependsOn {
			if _, err := fmt.Fprintf(writer, "  %s -> %s;\n", strconv.Quote(dependency), name); err != nil {
				return err
			}
		}
	}
	_, err := io.WriteString(writer, "}\n")
	return err
}
