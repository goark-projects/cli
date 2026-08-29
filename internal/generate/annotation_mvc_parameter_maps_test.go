package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCParameterMapsExist_shouldGenerateMapBindings(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("searchController")
type SearchController struct{}

//goark:get("/search")
//goark:request-param[params]
//goark:request-param[values]
//goark:request-header[headers]
//goark:request-header[headerValues]
func (c *SearchController) Search(params map[string]string, values map[string][]string, headers map[string]string, headerValues map[string][]string) map[string]any {
	return map[string]any{
		"params": params,
		"values": values,
		"headers": headers,
		"headerValues": headerValues,
	}
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
		`params, err := mvc.RequestParamMap(ctx)`,
		`values, err := mvc.RequestParamValuesMap(ctx)`,
		`headers, err := mvc.RequestHeaderMap(ctx)`,
		`headerValues, err := mvc.RequestHeaderValuesMap(ctx)`,
		`return controller.Search(params, values, headers, headerValues), nil`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc parameter map source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCParameterMapDeclaresSourceName_shouldReturnError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("searchController")
type SearchController struct{}

//goark:get("/search")
//goark:request-param[params]("tag")
func (c *SearchController) Search(params map[string]string) map[string]string {
	return params
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), `map parameter params must not declare name, value, defaultValue, or required=false`) {
		t.Fatalf("expected parameter map source error, got %v", err)
	}
}
