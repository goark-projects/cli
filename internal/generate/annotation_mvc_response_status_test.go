package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCResponseStatusExists_shouldUseResponseStatus(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:controller("adminController")
type AdminController struct{}

//goark:get("/jobs/accepted")
//goark:response-status(202)
func (c *AdminController) Accepted() map[string]string {
	return map[string]string{"state": "accepted"}
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
	if !strings.Contains(text, `mvc.GET("/jobs/accepted", mvc.Return[any](202`) {
		t.Fatalf("generated mvc response status source should use 202:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCResponseStatusOnResultReturn_shouldWrapHandler(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import arkweb "goark.dev/arkarta/web"

//goark:rest-controller("jobController")
type JobController struct{}

//goark:get("/jobs/accepted")
//goark:response-status(status=202)
func (c *JobController) Accepted(ctx *arkweb.Context) (arkweb.Result, error) {
	return nil, nil
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
	if !strings.Contains(text, `mvc.GET("/jobs/accepted", mvc.ResponseStatus(202, mvc.Handler`) {
		t.Fatalf("generated mvc result handler should be wrapped with response status:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCResponseStatusOnNoReturn_shouldWrapNoContentHandler(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("jobController")
type JobController struct{}

//goark:delete("/jobs/{id}")
//goark:response-status(code=202)
func (c *JobController) Delete() {}
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
	if !strings.Contains(text, `mvc.DELETE("/jobs/{id}", mvc.ResponseStatus(202, mvc.NoContent`) {
		t.Fatalf("generated mvc no-return handler should be wrapped with response status:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCResponseStatusDuplicatesMappingStatus_shouldReturnValidationError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:controller("adminController")
type AdminController struct{}

//goark:post("/jobs", status=201)
//goark:response-status(202)
func (c *AdminController) Create() map[string]string {
	return map[string]string{"state": "created"}
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), `must not declare both mapping status and response-status`) {
		t.Fatalf("expected response status conflict error, got %v", err)
	}
}
