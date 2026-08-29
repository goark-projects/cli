package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenControllerRequestMappingHasMethods_shouldInheritMethodsForImplicitRequestMapping(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("systemController")
//goark:request-mapping("/api", method="POST,TRACE")
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
		`mvc.POST("/api/probe", mvc.Return[any](200`,
		`mvc.TRACE("/api/probe", mvc.Return[any](200`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated inherited method route missing %q:\n%s", fragment, text)
		}
	}
	for _, fragment := range []string{
		`mvc.GET("/api/probe"`,
		`mvc.DELETE("/api/probe"`,
		`mvc.OPTIONS("/api/probe"`,
	} {
		if strings.Contains(text, fragment) {
			t.Fatalf("generated inherited method route must not include %q:\n%s", fragment, text)
		}
	}
	if strings.Count(text, "return controller.Probe()") != 2 {
		t.Fatalf("generated handler count is wrong:\n%s", text)
	}
}

func TestGenerateAnnotations_whenControllerAndHandlerMethodsDiffer_shouldCombineMethods(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("systemController")
//goark:request-mapping("/api", method="POST")
type SystemController struct{}

//goark:get("/probe")
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
		`mvc.GET("/api/probe", mvc.Return[any](200`,
		`mvc.POST("/api/probe", mvc.Return[any](200`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated combined method route missing %q:\n%s", fragment, text)
		}
	}
	if strings.Count(text, "return controller.Probe()") != 2 {
		t.Fatalf("generated handler count is wrong:\n%s", text)
	}
}
