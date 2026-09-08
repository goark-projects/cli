package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCRestControllerAdviceReturnsValue_shouldGenerateExceptionReturn(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

type UserNotFoundError struct {
	ID string
}

func (e *UserNotFoundError) Error() string {
	return "user " + e.ID + " not found"
}

//goark-web:rest-controller-advice("apiAdvice")
type APIAdvice struct{}

//goark-web:exception-handler
//goark-web:response-status(404)
func (a *APIAdvice) NotFound(err *UserNotFoundError) map[string]string {
	return map[string]string{"id": err.ID}
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
		`mvc.NewConfigurer().WithControllerAdvices(mvc.NewRestControllerAdvice("apiAdvice"`,
		`mvc.ExceptionReturnAs[*UserNotFoundError, any](404`,
		`func(_ *arkweb.Context, err *UserNotFoundError) any`,
		`return advice.NotFound(err)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated rest advice value source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCAdviceResponseBodyExists_shouldGenerateExceptionBody(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

type AccessDeniedError struct{}

func (e *AccessDeniedError) Error() string {
	return "access denied"
}

//goark-web:controller-advice("pageAdvice")
type PageAdvice struct{}

//goark-web:exception-handler
//goark-web:response-body
//goark-web:response-status(status=403)
func (a *PageAdvice) Denied(err *AccessDeniedError) string {
	return "denied"
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
		`mvc.NewConfigurer().WithControllerAdvices(mvc.NewControllerAdvice("pageAdvice"`,
		`mvc.ExceptionResponseBodyAs[*AccessDeniedError, any](403`,
		`return advice.Denied(err)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf(
				"generated controller advice response body source missing %q:\n%s",
				fragment,
				text,
			)
		}
	}
}

func TestGenerateAnnotations_whenMVCExceptionHandlerReturnsEntity_shouldGenerateEntity(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

import (
	"net/http"

	goweb "goark.dev/goark/web"
)

type UserNotFoundError struct {
	ID string
}

func (e *UserNotFoundError) Error() string {
	return "user " + e.ID + " not found"
}

//goark-web:rest-controller-advice("apiAdvice")
type APIAdvice struct{}

//goark-web:exception-handler
func (a *APIAdvice) NotFound(err *UserNotFoundError) goweb.ResponseEntity[map[string]string] {
	return goweb.Status(http.StatusNotFound, map[string]string{"id": err.ID})
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
		`mvc.ExceptionEntityAs[*UserNotFoundError, map[string]string](` +
			`func(_ *arkweb.Context, err *UserNotFoundError) ` +
			`goweb.ResponseEntity[map[string]string]`,
		`return advice.NotFound(err)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated exception response entity source missing %q:\n%s", fragment, text)
		}
	}
}
