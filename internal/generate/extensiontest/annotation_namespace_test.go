package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

type namespaceBinder struct {
	name  string
	calls *int
}

func (b namespaceBinder) BindAnnotation(
	_ *generate.AnnotationBindingContext, item generate.AnnotationItem,
) error {
	if item.HasAnnotation(b.name) {
		*b.calls++
	}
	return nil
}

func TestAnnotationNamespacesIsolateIdenticalNames(t *testing.T) {
	dir := t.TempDir()
	source := "package app\n//goark-example:service\ntype Example struct{}\n" +
		"//goark-other:service\ntype Other struct{}\n"
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var first, second int
	extensions := []generate.AnnotationExtension{
		{
			Descriptors: []generate.AnnotationDescriptor{{Name: "goark-example:service"}},
			Binder:      namespaceBinder{"goark-example:service", &first},
		},
		{
			Descriptors: []generate.AnnotationDescriptor{{Name: "goark-other:service"}},
			Binder:      namespaceBinder{"goark-other:service", &second},
		},
	}
	output, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{
		Dir: dir, Extensions: extensions,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first != 1 || second != 1 {
		t.Fatalf("同名注解分发错误: %d, %d", first, second)
	}
	if strings.Contains(string(output), "container.Register(registry") {
		t.Fatalf("领域 service 注解被核心容器误识别: %s", output)
	}
}

func TestWebAnnotationNamespaceValidation(t *testing.T) {
	for _, item := range []struct{ marker, want string }{
		{"//goark:controller", "moved to //goark-web:controller"},
		{"//goark-web:service", `unknown annotation "goark-web:service"`},
		{"//goark-web:unknown", `unknown annotation "goark-web:unknown"`},
	} {
		t.Run(item.marker, func(t *testing.T) {
			dir := t.TempDir()
			source := []byte("package app\n" + item.marker + "\ntype App struct{}\n")
			if err := os.WriteFile(filepath.Join(dir, "app.go"), source, 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
			if err == nil || !strings.Contains(err.Error(), item.want) {
				t.Fatalf("期望 %q，实际 %v", item.want, err)
			}
		})
	}
}

func TestIndependentORMAnnotationsRemainOwnedByORM(t *testing.T) {
	dir := t.TempDir()
	source := `package app
//goark:service
type Service struct{}
//goark-orm:tableName(value="users")
type User struct{}
//goark-orm:mapper(namespace="users")
type Mapper interface {
	//goark-orm:delete("delete from users")
	Delete() error
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), `container.Register(registry, "service"`) {
		t.Fatalf("混合包未生成核心 Bean: %s", output)
	}
}

func TestDuplicateNamespacedDescriptorIsRejected(t *testing.T) {
	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{
		Dir: t.TempDir(),
		Extensions: []generate.AnnotationExtension{{
			Descriptors: []generate.AnnotationDescriptor{{Name: "goark-web:get"}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate annotation descriptor") {
		t.Fatalf("重复领域注解未被拒绝: %v", err)
	}
}
