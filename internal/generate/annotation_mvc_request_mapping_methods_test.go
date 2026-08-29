package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenRequestMappingHasNoMethod_shouldGenerateDefaultMethodRoutes(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("systemController")
type SystemController struct{}

//goark:request-mapping("/probe")
func (c *SystemController) Probe() string {
	return "ok"
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
	for _, fragment := range []string{
		`mvc.DELETE("/probe", mvc.Return[any](200`,
		`mvc.GET("/probe", mvc.Return[any](200`,
		`mvc.HEAD("/probe", mvc.Return[any](200`,
		`mvc.OPTIONS("/probe", mvc.Return[any](200`,
		`mvc.PATCH("/probe", mvc.Return[any](200`,
		`mvc.POST("/probe", mvc.Return[any](200`,
		`mvc.PUT("/probe", mvc.Return[any](200`,
		`mvc.TRACE("/probe", mvc.Return[any](200`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated default request mapping route missing %q:\n%s", fragment, text)
		}
	}
	if strings.Count(text, "return controller.Probe()") != 8 {
		t.Fatalf("generated handler count is wrong:\n%s", text)
	}
}

func TestGenerateAnnotations_whenRequestMappingHasMultipleMethods_shouldGenerateRoutePerMethod(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("jobsController")
//goark:request-mapping("/api")
type JobsController struct{}

//goark:request-mapping("/jobs", method="GET,POST")
func (c *JobsController) Jobs() []string {
	return []string{"sync"}
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
	for _, fragment := range []string{
		`mvc.GET("/api/jobs", mvc.Return[any](200`,
		`mvc.POST("/api/jobs", mvc.Return[any](200`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated multi-method route missing %q:\n%s", fragment, text)
		}
	}
	if strings.Count(text, "return controller.Jobs()") != 2 {
		t.Fatalf("generated handler count is wrong:\n%s", text)
	}
}

func TestGenerateAnnotations_whenTraceMappingExists_shouldGenerateTraceRoute(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("diagnosticController")
type DiagnosticController struct{}

//goark:trace("/diagnostics")
func (c *DiagnosticController) Diagnostics() string {
	return "ok"
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
	if !strings.Contains(text, `mvc.TRACE("/diagnostics", mvc.Return[any](200`) {
		t.Fatalf("generated trace route missing:\n%s", text)
	}
}
