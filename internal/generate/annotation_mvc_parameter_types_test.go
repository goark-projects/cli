package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCExtendedParameterTypesExist_shouldGenerateParameterBindings(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import "time"

//goark:rest-controller("adminController")
type AdminController struct{}

//goark:get("/items/{ids}")
//goark:path-variable[ids]
//goark:request-param[tags](name="tag")
//goark:request-param[ratio]
//goark:request-param[at]
//goark:request-header[roles](name="X-Role")
//goark:cookie-value[flags](name="flag", required=false)
//goark:matrix-variable[years](name="year")
//goark:request-attribute[score]
//goark:session-attribute[sessionAt]
func (c *AdminController) Search(ids []int, tags []string, ratio float64, at time.Time, roles []string, flags []bool, years []int64, score float64, sessionAt time.Time) map[string]any {
	return map[string]any{
		"ids": ids,
		"tags": tags,
		"ratio": ratio,
		"at": at,
		"roles": roles,
		"flags": flags,
		"years": years,
		"score": score,
		"sessionAt": sessionAt,
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
		`ids, err := mvc.PathInts(ctx, "ids")`,
		`tags, err := mvc.RequestParamStrings(ctx, "tag")`,
		`ratio, err := mvc.RequestParamFloat64(ctx, "ratio")`,
		`at, err := mvc.RequestParamTime(ctx, "at")`,
		`roles, err := mvc.RequestHeaderStrings(ctx, "X-Role")`,
		`flags, err := mvc.CookieValueBools(ctx, "flag", mvc.WithRequired(false))`,
		`years, err := mvc.MatrixVariableInt64s(ctx, "year")`,
		`score, err := mvc.RequestAttributeFloat64(ctx, "score")`,
		`sessionAt, err := mvc.SessionAttributeTime(ctx, "sessionAt")`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc parameter binding source missing %q:\n%s", fragment, text)
		}
	}
}
