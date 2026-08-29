package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenControllerRequestMappingHasConditions_shouldGenerateControllerOptions(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("jobsController")
//goark:request-mapping("/api", consumes="application/json", produces="application/json", params="tenant=admin", headers="X-Tenant=admin")
type JobsController struct{}

//goark:post("/jobs", consumes="application/vnd.goark.job+json", produces="application/vnd.goark.job+json", params="mode=fast", headers="X-Route=enabled")
func (c *JobsController) Create() map[string]string {
	return map[string]string{"status": "created"}
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
		`mvc.POST("/api/jobs", mvc.Return[any](201`,
		`mvc.WithConsumes("application/vnd.goark.job+json")`,
		`mvc.WithProduces("application/vnd.goark.job+json")`,
		`mvc.WithParams("mode=fast")`,
		`mvc.WithHeaders("X-Route=enabled")`,
		`).WithConsumes("application/json").WithProduces("application/json").WithParams("tenant=admin").WithHeaders("X-Tenant=admin")`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated controller condition source missing %q:\n%s", fragment, text)
		}
	}
}
