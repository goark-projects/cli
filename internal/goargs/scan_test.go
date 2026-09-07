package goargs

import (
	"reflect"
	"slices"
	"testing"
)

func TestScanPreservesBoundaries(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{"透传", []string{"-tags=a", "./...", "-args", "-C=b"}, []string{"-tags", "", ""}},
		{"双横线", []string{"--", "-mod=mod"}, []string{""}},
		{"构建参数值", []string{"-ldflags", "-mod=mod", "-C", "dir"}, []string{"-ldflags", "-C"}},
		{"测试参数值", []string{"-run", "-C=pattern", "./..."}, []string{"-run", ""}},
		{"测试前缀", []string{"-test.run", "-mod=mod"}, []string{"-test.run"}},
		{"分隔符作为值", []string{"-ldflags", "--", "-mod=readonly"}, []string{"-ldflags", "-mod"}},
		{"空值", []string{"-C="}, []string{"-C"}},
		{"缺值", []string{"-C"}, []string{"-C"}},
		{"双横线选项", []string{"--mod=readonly"}, []string{"-mod"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := slices.Clone(test.args)
			var names, rebuilt []string
			for entry := range Scan(test.args) {
				names = append(names, entry.Name)
				rebuilt = append(rebuilt, entry.Args...)
			}
			if !reflect.DeepEqual(names, test.want) || !reflect.DeepEqual(rebuilt, before) ||
				!reflect.DeepEqual(test.args, before) {
				t.Fatalf("扫描不保持边界: names=%q, args=%q", names, rebuilt)
			}
		})
	}
}

func TestParseFlags(t *testing.T) {
	for _, test := range []struct {
		input string
		want  []string
	}{
		{`"-mod=readonly"`, []string{"-mod=readonly"}},
		{`'-ldflags=-X main.value= -mod=readonly' -tags=a`,
			[]string{"-ldflags=-X main.value= -mod=readonly", "-tags=a"}},
		{" \t\r\n", nil},
		{`""`, []string{""}},
	} {
		got, err := ParseFlags(test.input)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("解析 %q: got=%q, err=%v", test.input, got, err)
		}
	}
	if _, err := ParseFlags(`"-mod=readonly`); err == nil {
		t.Fatal("未拒绝缺失的结束引号")
	}
}

func FuzzScanPreservesArguments(f *testing.F) {
	f.Add("-C", "--", "-mod=mod")
	f.Add("-args", "-C=value", "-tags=test")
	f.Fuzz(func(t *testing.T, first, second, third string) {
		args := []string{first, second, third}
		var rebuilt []string
		for entry := range Scan(args) {
			rebuilt = append(rebuilt, entry.Args...)
		}
		if !reflect.DeepEqual(args, rebuilt) {
			t.Fatalf("参数发生丢失或重排: %q => %q", args, rebuilt)
		}
	})
}
