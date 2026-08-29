package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCRequestEntityAnnotated_shouldGenerateBindRequestEntity(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import (
	arkweb "goark.dev/arkarta/web"
	goweb "goark.dev/goark/web"
)

type CreateJobRequest struct {
	Name string ` + "`json:\"name\"`" + `
}

type Job struct {
	Name string ` + "`json:\"name\"`" + `
}

//goark:rest-controller("adminController")
type AdminController struct{}

//goark:post("/jobs", status=202)
//goark:request-entity[request]
func (c *AdminController) Create(ctx *arkweb.Context, request goweb.RequestEntity[CreateJobRequest]) (Job, error) {
	body, _ := request.Body()
	return Job{Name: body.Name}, nil
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
		`mvc.POST("/jobs", mvc.BindRequestEntity[CreateJobRequest, any](202`,
		`func(ctx *arkweb.Context, request goweb.RequestEntity[CreateJobRequest]) (any, error)`,
		`return controller.Create(ctx, request)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated request entity source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCRequestEntityTypeExists_shouldInferBinding(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import (
	"net/http"

	goweb "goark.dev/goark/web"
)

type CreateJobRequest struct {
	Name string ` + "`json:\"name\"`" + `
}

type Job struct {
	Name string ` + "`json:\"name\"`" + `
}

//goark:rest-controller("adminController")
type AdminController struct{}

//goark:post("/jobs")
func (c *AdminController) Create(request goweb.RequestEntity[CreateJobRequest]) (goweb.ResponseEntity[Job], error) {
	body, _ := request.Body()
	return goweb.Status(http.StatusAccepted, Job{Name: body.Name}), nil
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
		`mvc.POST("/jobs", mvc.BindRequestEntityEntity[CreateJobRequest, Job](func(ctx *arkweb.Context, request goweb.RequestEntity[CreateJobRequest]) (goweb.ResponseEntity[Job], error)`,
		`return controller.Create(request)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated inferred request entity source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCRequestEntityValidatedExists_shouldGenerateGroups(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import goweb "goark.dev/goark/web"

type CreateJobRequest struct {
	Name string ` + "`json:\"name\" arkarta:\"required\" arkarta-groups:\"create\"`" + `
	Code string ` + "`json:\"code\" arkarta:\"required\"`" + `
}

//goark:rest-controller("adminController")
type AdminController struct{}

//goark:post("/jobs")
//goark:validated("create")
func (c *AdminController) Create(request goweb.RequestEntity[CreateJobRequest]) map[string]string {
	body, _ := request.Body()
	return map[string]string{"name": body.Name}
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
		`mvc.BindRequestEntityGroups[CreateJobRequest, any](201`,
		`func(ctx *arkweb.Context, request goweb.RequestEntity[CreateJobRequest]) (any, error)`,
		`}, "create")`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated request entity validation groups source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCRequestEntityAnnotationTargetsPlainType_shouldReturnValidationError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type CreateJobRequest struct{}

//goark:rest-controller("adminController")
type AdminController struct{}

//goark:post("/jobs")
//goark:request-entity[input]
func (c *AdminController) Create(input CreateJobRequest) map[string]string {
	return map[string]string{}
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "request entity parameter input must be goark.dev/goark/web.RequestEntity[T]") {
		t.Fatalf("expected request entity validation error, got %v", err)
	}
}
