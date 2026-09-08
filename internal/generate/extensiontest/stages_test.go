package generate_test

import (
	"goark.dev/cli/internal/generate"
	"os"
	"path/filepath"
	"testing"
)

type stageProbe struct{ bindings, renders int }

func (p *stageProbe) BindAnnotation(
	_ *generate.AnnotationBindingContext, _ generate.AnnotationItem,
) error {
	p.bindings++
	return nil
}
func (p *stageProbe) GenerateAnnotation(_ *generate.AnnotationGenerationContext) error {
	p.renders++
	return nil
}

func stageSpec(t *testing.T, source string, probe *stageProbe) generate.AnnotationScanSpec {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return generate.AnnotationScanSpec{Dir: dir, Extensions: []generate.AnnotationExtension{{
		Name: "probe", Binder: probe, Generator: probe,
		Descriptors: []generate.AnnotationDescriptor{{Name: "probe"}},
	}}}
}

func TestBatchValidatesAllPackagesBeforeBinding(t *testing.T) {
	probe := &stageProbe{}
	good := stageSpec(t, "package app\n//goark:probe\ntype Good struct{}\n", probe)
	bad := stageSpec(t, "package app\n//goark:unknown\ntype Bad struct{}\n", probe)
	_, err := generate.PrepareAnnotationPlans([]generate.AnnotationScanSpec{good, bad})
	if err == nil || probe.bindings != 0 || probe.renders != 0 {
		t.Fatalf("整批校验屏障失效: %v, %+v", err, probe)
	}
}

func TestPlanDoesNotRenderUntilRequested(t *testing.T) {
	probe := &stageProbe{}
	spec := stageSpec(t, "package app\n//goark:probe\ntype Good struct{}\n", probe)
	plans, err := generate.PrepareAnnotationPlans([]generate.AnnotationScanSpec{spec})
	if err != nil {
		t.Fatal(err)
	}
	if probe.bindings != 1 || probe.renders != 0 {
		t.Fatalf("阶段顺序错误: %+v", probe)
	}
	if _, err := plans[0].RenderFiles(); err != nil {
		t.Fatal(err)
	}
	if probe.renders != 1 {
		t.Fatalf("未执行渲染: %+v", probe)
	}
}

func TestInvalidPlanReturnsError(t *testing.T) {
	for _, plan := range []*generate.AnnotationPlan{nil, {}} {
		func() {
			defer func() {
				if value := recover(); value != nil {
					t.Errorf("无效计划发生 panic: %v", value)
				}
			}()
			if _, err := plan.RenderFiles(); err == nil {
				t.Error("无效计划应返回错误")
			}
		}()
	}
}

func BenchmarkAnnotationBatch(b *testing.B) {
	specs := make([]generate.AnnotationScanSpec, 8)
	for i := range specs {
		dir := b.TempDir()
		source := []byte("package app\n//goark:service\ntype Service struct{}\n")
		if err := os.WriteFile(filepath.Join(dir, "app.go"), source, 0o644); err != nil {
			b.Fatal(err)
		}
		specs[i] = generate.AnnotationScanSpec{Dir: dir}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plans, err := generate.PrepareAnnotationPlans(specs)
		if err != nil {
			b.Fatal(err)
		}
		for _, plan := range plans {
			if _, err := plan.RenderFiles(); err != nil {
				b.Fatal(err)
			}
		}
	}
}
