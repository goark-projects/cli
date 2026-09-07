package goconfig

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"goark.dev/cli/internal/processrun"
)

type responseRunner struct {
	output string
	err    error
}

func (r responseRunner) Run(request processrun.Request) error {
	_, _ = fmt.Fprint(request.Out, r.output)
	return r.err
}

func TestModuleEnvironmentValidatesResponse(t *testing.T) {
	for _, test := range []struct {
		name   string
		output string
		err    error
		valid  bool
	}{
		{name: "空模式", output: `{"GOFLAGS":"","GOWORK":""}`, valid: true},
		{name: "无输出"},
		{name: "缺少字段", output: `{"GOFLAGS":""}`},
		{name: "空对象字段", output: `{"GOFLAGS":null,"GOWORK":""}`},
		{name: "进程错误", err: context.Canceled},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := ModuleEnvironment(context.Background(), responseRunner{
				output: test.output, err: test.err,
			}, t.TempDir(), nil)
			if (err == nil) != test.valid {
				t.Fatalf("配置有效性错误: %v", err)
			}
			if test.err != nil && !errors.Is(err, test.err) {
				t.Fatalf("错误链丢失: %v", err)
			}
		})
	}
}
