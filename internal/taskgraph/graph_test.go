package taskgraph

import (
	"reflect"
	"strings"
	"testing"

	"goark.dev/cli/internal/buildspec"
)

func TestNew_whenGraphIsValid_shouldReturnDeterministicTopologicalClosure(t *testing.T) {
	graph, err := New(map[string]buildspec.Task{
		"package": {Type: buildspec.TaskTypeGroup, DependsOn: []string{"test", "generate"}},
		"test":    {Type: buildspec.TaskTypeGo, Args: []string{"test", "./..."}, DependsOn: []string{"generate"}},
		"generate": {
			Type: buildspec.TaskTypeExec,
			Tool: "generator",
		},
	})
	if err != nil {
		t.Fatalf("创建任务图失败: %v", err)
	}

	order, err := graph.Order([]string{"package"})
	if err != nil {
		t.Fatalf("计算任务闭包失败: %v", err)
	}
	want := []string{"generate", "test", "package"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("执行顺序 = %#v, want %#v", order, want)
	}
	if !reflect.DeepEqual(graph.Dependents("generate"), []string{"package", "test"}) {
		t.Fatalf("反向依赖 = %#v", graph.Dependents("generate"))
	}
}

func TestNew_whenDependencyGraphIsInvalid_shouldReject(t *testing.T) {
	tests := []struct {
		name  string
		tasks map[string]buildspec.Task
		want  []string
	}{
		{
			name: "missing dependency",
			tasks: map[string]buildspec.Task{
				"one": {DependsOn: []string{"missing"}},
			},
			want: []string{"one", "missing"},
		},
		{
			name: "duplicate dependency",
			tasks: map[string]buildspec.Task{
				"one": {DependsOn: []string{"two", "two"}},
				"two": {},
			},
			want: []string{"one", "two", "重复"},
		},
		{
			name: "cycle",
			tasks: map[string]buildspec.Task{
				"one":   {DependsOn: []string{"two"}},
				"two":   {DependsOn: []string{"three"}},
				"three": {DependsOn: []string{"one"}},
			},
			want: []string{"one", "two", "three", "循环"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.tasks)
			if err == nil {
				t.Fatal("非法任务图必须失败")
			}
			for _, fragment := range tt.want {
				if !strings.Contains(err.Error(), fragment) {
					t.Fatalf("错误 %q 缺少 %q", err, fragment)
				}
			}
		})
	}
}

func TestNew_whenOutputsMayOverlap_shouldReject(t *testing.T) {
	tests := []struct {
		name   string
		first  string
		second string
	}{
		{name: "same file", first: "build/app", second: "./build/app"},
		{name: "parent directory", first: "build", second: "build/app"},
		{name: "matching glob", first: "generated/**/*.go", second: "generated/app/*.go"},
		{name: "glob matches fixed path", first: "generated/*/one.go", second: "generated/admin/one.go"},
		{name: "variable may match static output", first: "${env:OUTPUT}/*.go", second: "generated/*.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(map[string]buildspec.Task{
				"first":  {Outputs: []string{tt.first}},
				"second": {Outputs: []string{tt.second}},
			})
			if err == nil || !strings.Contains(err.Error(), "输出冲突") {
				t.Fatalf("错误 = %v", err)
			}
		})
	}
}

func TestNew_whenOutputsAreDisjoint_shouldAccept(t *testing.T) {
	tests := []struct {
		name   string
		first  string
		second string
	}{
		{name: "different directories", first: "generated/first/*.go", second: "generated/second/*.go"},
		{name: "different files below wildcard", first: "generated/*/one.go", second: "generated/*/two.go"},
		{name: "glob excludes fixed path", first: "generated/*/one.go", second: "generated/admin/two.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(map[string]buildspec.Task{
				"first":  {Outputs: []string{tt.first}},
				"second": {Outputs: []string{tt.second}},
			})
			if err != nil {
				t.Fatalf("不相交输出被拒绝: %v", err)
			}
		})
	}
}

func TestOrder_whenTargetDoesNotExist_shouldReject(t *testing.T) {
	graph, err := New(map[string]buildspec.Task{"one": {}})
	if err != nil {
		t.Fatalf("创建任务图失败: %v", err)
	}
	_, err = graph.Order([]string{"missing"})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("错误 = %v", err)
	}
}
