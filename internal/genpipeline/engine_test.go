package genpipeline

import (
	"errors"
	"reflect"
	"testing"
)

func TestExecuteOrderAndFailure(t *testing.T) {
	failure := errors.New("invalid model")
	var calls []string
	err := Execute(
		Step{Name: "scan", Run: func() error { calls = append(calls, "scan"); return nil }},
		Step{Name: "validate", Run: func() error { calls = append(calls, "validate"); return failure }},
		Step{Name: "render", Run: func() error { t.Fatal("校验失败后不应渲染"); return nil }},
	)
	if !errors.Is(err, failure) || !reflect.DeepEqual(calls, []string{"scan", "validate"}) {
		t.Fatalf("阶段或错误链异常: %v, %v", calls, err)
	}
}

func TestExecuteRejectsInvalidChainBeforeRunning(t *testing.T) {
	for _, invalid := range []Step{{Name: "scan", Run: func() error { return nil }}, {Name: "bad"}} {
		err := Execute(Step{Name: "scan", Run: func() error {
			t.Fatal("非法阶段链不应执行任何阶段")
			return nil
		}}, invalid)
		if err == nil {
			t.Fatal("应拒绝重复名称或空处理器")
		}
	}
}
