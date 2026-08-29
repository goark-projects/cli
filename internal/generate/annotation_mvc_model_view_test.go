package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCModelAndViewReturnExists_shouldGenerateReturnHandler(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import mvc "goark.dev/goark/web/mvc"

//goark:controller("pageController")
type PageController struct{}

//goark:get("/pages/home")
func (c *PageController) Home() (mvc.ModelAndView, error) {
	model := mvc.NewModel().AddAttribute("Title", "Home")
	return mvc.NewModelAndView("pages/home", model), nil
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
	if !strings.Contains(text, `mvc.GET("/pages/home", mvc.Return[any](200`) {
		t.Fatalf("generated mvc model and view source should use return handler:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCModelParameterAndViewNameReturnExist_shouldRenderModelAndView(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import mvc "goark.dev/goark/web/mvc"

//goark:controller("pageController")
type PageController struct{}

//goark:get("/pages/home")
func (c *PageController) Home(model *mvc.Model) string {
	model.AddAttribute("Title", "Home")
	return "pages/home"
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
		"model := mvc.NewModel()",
		"viewName := controller.Home(&model)",
		"return mvc.NewModelAndView(viewName, model, mvc.WithViewStatus(200)), nil",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc model parameter source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCModelParameterAndNoReturnExist_shouldInferViewName(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import mvc "goark.dev/goark/web/mvc"

//goark:controller("pageController")
type PageController struct{}

//goark:get("/pages/home")
func (c *PageController) Home(model *mvc.Model) {
	model.AddAttribute("Title", "Home")
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
		"model := mvc.NewModel()",
		"controller.Home(&model)",
		"return mvc.NewModelAndView(\"\", model, mvc.WithViewStatus(200)), nil",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc no-return model source missing %q:\n%s", fragment, text)
		}
	}
}
