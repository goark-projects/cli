package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCModelAttributeMethodExists_shouldGenerateInitializer(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import (
	arkweb "goark.dev/arkarta/web"
	"goark.dev/goark/web/mvc"
)

//goark:controller("pageController")
//goark:request-mapping("/pages")
type PageController struct{}

//goark:model-attribute("AppName")
func (c *PageController) AppName(ctx *arkweb.Context) (string, error) {
	return "Goark", nil
}

//goark:get("/home")
func (c *PageController) Home() string {
	return "home"
}

//goark:get("/dashboard")
func (c *PageController) Dashboard() (mvc.Model, error) {
	return mvc.NewModel().AddAttribute("Title", "Dashboard"), nil
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
		`mvc.NewController("pageController"`,
		`).WithModelAttributes(`,
		`mvc.ModelAttributeValue[string]("AppName", func(ctx *arkweb.Context) (string, error) {`,
		`return controller.AppName(ctx)`,
		`mvc.GET("/pages/home", mvc.Return[any](200`,
		`mvc.GET("/pages/dashboard", mvc.Return[any](200`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc model attribute method source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCModelAttributeMethodHasNoName_shouldReturnValidationError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:controller("pageController")
type PageController struct{}

//goark:model-attribute
func (c *PageController) AppName() string {
	return "Goark"
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), `requires model attribute name`) {
		t.Fatalf("expected mvc model attribute method validation error, got %v", err)
	}
}

func TestGenerateAnnotations_whenMVCModelAttributeMethodHasDuplicateAnnotations_shouldReturnValidationError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:controller("pageController")
type PageController struct{}

//goark:model-attribute("AppName")
//goark:model-attribute("Title")
func (c *PageController) AppName() string {
	return "Goark"
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), `multiple model-attribute annotations`) {
		t.Fatalf("expected duplicate mvc model attribute method validation error, got %v", err)
	}
}
