package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCMappingHasMultiplePaths_shouldGenerateRoutePerPath(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

//goark-web:rest-controller("adminController")
//goark-web:request-mapping("/api")
type AdminController struct{}

//goark-web:get("/users", "/members")
func (c *AdminController) List() []string {
	return []string{"goark"}
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
		`mvc.GET("/api/members", mvc.Return[any](200`,
		`mvc.GET("/api/users", mvc.Return[any](200`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated multi-path route missing %q:\n%s", fragment, text)
		}
	}
	if strings.Count(text, "return controller.List()") != 2 {
		t.Fatalf("generated handler count is wrong:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCTypeAndMethodHavePaths_shouldGenerateCartesianRoutes(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

//goark-web:rest-controller("adminController")
//goark-web:request-mapping("/api", "/internal")
type AdminController struct{}

//goark-web:request-mapping("/health", "/ready", method="GET")
func (c *AdminController) Health() string {
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
	for _, fragment := range []string{
		`mvc.GET("/api/health", mvc.Return[any](200`,
		`mvc.GET("/api/ready", mvc.Return[any](200`,
		`mvc.GET("/internal/health", mvc.Return[any](200`,
		`mvc.GET("/internal/ready", mvc.Return[any](200`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated cartesian route missing %q:\n%s", fragment, text)
		}
	}
	if strings.Count(text, "return controller.Health()") != 4 {
		t.Fatalf("generated cartesian handler count is wrong:\n%s", text)
	}
}
