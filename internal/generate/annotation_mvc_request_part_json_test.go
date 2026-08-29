package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCRequestPartStructExists_shouldGenerateJSONPartBinding(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import arkweb "goark.dev/arkarta/web"

type UploadMetadata struct {
	Name string ` + "`json:\"name\" arkarta:\"required\"`" + `
}

//goark:rest-controller("uploadController")
type UploadController struct{}

//goark:post("/uploads", consumes="multipart/form-data")
//goark:request-part[metadata]("metadata")
func (c *UploadController) Upload(ctx *arkweb.Context, metadata UploadMetadata) (map[string]string, error) {
	return map[string]string{"name": metadata.Name}, nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	expected := []string{
		`mvc.WithConsumes("multipart/form-data")`,
		`metadata, err := mvc.RequestPartJSON[UploadMetadata](ctx, "metadata")`,
		`return controller.Upload(ctx, metadata)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated request part JSON source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCRequestPartStructValidatedExists_shouldGenerateValidatedJSONPartBinding(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type UploadMetadata struct {
	Name string ` + "`json:\"name\" arkarta:\"required\" arkarta-groups:\"create\"`" + `
}

//goark:rest-controller("uploadController")
type UploadController struct{}

//goark:post("/uploads", consumes="multipart/form-data")
//goark:request-part[metadata](name="metadata", required=false)
//goark:validated("create")
func (c *UploadController) Upload(metadata UploadMetadata) (map[string]string, error) {
	return map[string]string{"name": metadata.Name}, nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	expected := []string{
		`metadata, err := mvc.ValidatedRequestPartJSON[UploadMetadata](ctx, "metadata", []string{"create"}, mvc.WithRequired(false))`,
		`return controller.Upload(metadata)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated validated request part JSON source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCRequestPartFileExists_shouldKeepPartBinding(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import servletmultipart "goark.dev/arkarta/servlet/multipart"

//goark:rest-controller("uploadController")
type UploadController struct{}

//goark:post("/uploads", consumes="multipart/form-data")
//goark:request-part[file]("file")
func (c *UploadController) Upload(file servletmultipart.Part) (map[string]string, error) {
	return map[string]string{"file": file.SubmittedFileName()}, nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	if !strings.Contains(text, `file, err := mvc.RequestPart(ctx, "file")`) {
		t.Fatalf("generated file request part source should keep RequestPart:\n%s", text)
	}
	if strings.Contains(text, `mvc.RequestPartJSON[servletmultipart.Part]`) {
		t.Fatalf("generated file request part source must not use JSON part binding:\n%s", text)
	}
}
