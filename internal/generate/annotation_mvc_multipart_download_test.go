package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCMultipartBodyExists_shouldGenerateBindMultipartHandler(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import (
	arkweb "goark.dev/arkarta/web"
	servletmultipart "goark.dev/arkarta/servlet/multipart"
)

type UploadRequest struct {
	Title string ` + "`form:\"title\"`" + `
	File servletmultipart.Part ` + "`multipart:\"file\"`" + `
}

//goark:controller("uploadController")
type UploadController struct{}

//goark:post("/uploads")
//goark:multipart-body[input]
func (c *UploadController) Create(ctx *arkweb.Context, input UploadRequest) (map[string]string, error) {
	return map[string]string{"title": input.Title}, nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "upload.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	expected := []string{
		"mvc.POST(\"/uploads\", mvc.BindMultipart[UploadRequest, any](201",
		"func(ctx *arkweb.Context, input UploadRequest) (any, error)",
		"return controller.Create(ctx, input)",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc multipart source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCDownloadResultExists_shouldGenerateResultHandler(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import (
	"strings"

	arkweb "goark.dev/arkarta/web"
	goweb "goark.dev/goark/web"
)

//goark:controller("reportController")
type ReportController struct{}

//goark:get("/reports/today")
func (c *ReportController) Download(ctx *arkweb.Context) (goweb.DownloadResult, error) {
	return goweb.Attachment("today.csv", strings.NewReader("id,name\n1,goark\n")), nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "report.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	expected := []string{
		"mvc.GET(\"/reports/today\", mvc.Handler(func(ctx *arkweb.Context) (arkweb.Result, error) {",
		"return controller.Download(ctx)",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc download source missing %q:\n%s", fragment, text)
		}
	}
	if strings.Contains(text, "mvc.JSON[any]") {
		t.Fatalf("download result must not be wrapped as JSON value:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCMultipartBodySelectorMissing_shouldReturnValidationError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:controller
type UploadController struct{}

//goark:post("/uploads")
//goark:multipart-body
func (c *UploadController) Create(input string) string {
	return input
}
`
	if err := os.WriteFile(filepath.Join(dir, "upload.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "requires parameter selector") {
		t.Fatalf("expected multipart body selector validation error, got %v", err)
	}
}
