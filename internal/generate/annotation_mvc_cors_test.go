package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCCrossOriginExists_shouldGenerateCORSOptions(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("apiController")
//goark:request-mapping("/api")
//goark:cross-origin(origins="https://admin.example.com", allowedHeaders="X-Request-ID,Content-Type", allowCredentials=true, maxAge="30m")
type APIController struct{}

//goark:get("/status")
//goark:cross-origin(origins="https://route.example.com", methods="GET", exposedHeaders="X-Trace-ID")
func (c *APIController) Status() string {
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
	expected := []string{
		`"goark.dev/goark/web/cors"`,
		`"time"`,
		`mvc.NewRestController("apiController"`,
		`mvc.WithCrossOrigin(cors.Config{`,
		`AllowedOrigins: []string{"https://route.example.com"}`,
		`AllowedMethods: []string{"GET"}`,
		`ExposedHeaders: []string{"X-Trace-ID"}`,
		`.WithCrossOrigin(cors.Config{`,
		`AllowedOrigins: []string{"https://admin.example.com"}`,
		`AllowedHeaders: []string{"X-Request-ID", "Content-Type"}`,
		`AllowCredentials: true`,
		`MaxAge: time.Duration(1800000000000)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc cross-origin source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenEmptyMVCCrossOriginExists_shouldGenerateDefaultCORSOption(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("apiController")
type APIController struct{}

//goark:get("/status")
//goark:cross-origin
func (c *APIController) Status() string {
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
	if !strings.Contains(text, `mvc.WithCrossOrigin(cors.Config{})`) {
		t.Fatalf("generated mvc cross-origin defaults missing:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCCrossOriginMethodHasNoRoute_shouldReturnValidationError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:rest-controller("apiController")
type APIController struct{}

//goark:cross-origin(origins="https://admin.example.com")
func (c *APIController) Status() string {
	return "UP"
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), `annotation "cross-origin" requires mvc route method target`) {
		t.Fatalf("expected mvc cross-origin route validation error, got %v", err)
	}
}
