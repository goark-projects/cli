package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCResponseEntityExists_shouldGenerateResultHandler(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import (
	"net/http"

	arkweb "goark.dev/arkarta/web"
	goweb "goark.dev/goark/web"
)

//goark-web:controller("adminController")
//goark-web:request-mapping("/admin")
type AdminController struct{}

//goark-web:get("/jobs/{id}")
func (c *AdminController) Detail(
	ctx *arkweb.Context,
) (goweb.ResponseEntity[map[string]string], error) {
	return goweb.Status(http.StatusAccepted, map[string]string{"id": ctx.PathValue("id")}), nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	assertGeneratedSourceParses(t, generated)
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	expected := []string{
		"mvc.GET(\"/admin/jobs/{id}\", mvc.Handler(func(ctx *arkweb.Context) (arkweb.Result, error) {",
		"return controller.Detail(ctx)",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc response entity source missing %q:\n%s", fragment, text)
		}
	}
	if strings.Contains(text, "mvc.JSON[any]") {
		t.Fatalf("response entity must not be wrapped as JSON value:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCRequestBodyReturnsResponseEntity_shouldBindAndReturnResult(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

import (
	"net/http"

	arkweb "goark.dev/arkarta/web"
	goweb "goark.dev/goark/web"
)

type CreateJobRequest struct {
	Name string ` + "`json:\"name\"`" + `
}

type Job struct {
	Name string ` + "`json:\"name\"`" + `
}

//goark-web:controller("adminController")
//goark-web:request-mapping("/admin")
type AdminController struct{}

//goark-web:post("/jobs")
//goark-web:request-body[input]
func (c *AdminController) Create(
	ctx *arkweb.Context,
	input CreateJobRequest,
) (goweb.ResponseEntity[Job], error) {
	return goweb.Status(http.StatusCreated, Job{Name: input.Name}), nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	assertGeneratedSourceParses(t, generated)
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	expected := []string{
		"mvc.POST(\"/admin/jobs\", mvc.Handler(func(ctx *arkweb.Context) (arkweb.Result, error) {",
		"var input CreateJobRequest",
		"if err := ctx.BindAndValidateJSON(&input); err != nil {",
		"return controller.Create(ctx, input)",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc response entity bind source missing %q:\n%s", fragment, text)
		}
	}
	if strings.Contains(text, "mvc.BindJSON") || strings.Contains(text, "mvc.JSON[any]") {
		t.Fatalf("request body response entity must not be JSON-value wrapped:\n%s", text)
	}
}
