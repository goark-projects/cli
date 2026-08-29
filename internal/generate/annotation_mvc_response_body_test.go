package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCResponseBodyExists_shouldWriteReturnValueAsBody(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:controller("adminController")
type AdminController struct{}

//goark:get("/admin/status")
//goark:response-body
func (c *AdminController) Status() string {
	return "UP"
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
	if !strings.Contains(text, `mvc.GET("/admin/status", mvc.ResponseBody[any](200`) {
		t.Fatalf("generated mvc response body source should use ResponseBody:\n%s", text)
	}
	if strings.Contains(text, `mvc.GET("/admin/status", mvc.Return[any]`) {
		t.Fatalf("generated mvc response body source must not use view-aware Return:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCRestControllerResponseBodyExists_shouldKeepExplicitBodyWrapper(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("apiController")
type APIController struct{}

//goark:get("/api/status")
//goark:response-body
func (c *APIController) Status() (map[string]string, error) {
	return map[string]string{"status": "UP"}, nil
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
	if !strings.Contains(text, `mvc.GET("/api/status", mvc.ResponseBody[any](200`) {
		t.Fatalf("generated rest controller response body source should keep explicit ResponseBody:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCResponseBodyUsesModel_shouldReturnValidationError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import "goark.dev/goark/web/mvc"

//goark:controller("pageController")
type PageController struct{}

//goark:get("/pages/home")
//goark:response-body
func (c *PageController) Home(model *mvc.Model) string {
	model.Add("title", "Home")
	return "home"
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), `response-body must not be used with *mvc.Model`) {
		t.Fatalf("expected response body model validation error, got %v", err)
	}
}
