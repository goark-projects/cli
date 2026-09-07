package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCControllerAdviceExists_shouldGenerateExceptionHandlers(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

import (
	"net/http"

	arkweb "goark.dev/arkarta/web"
)

type UserNotFoundError struct {
	ID string
}

func (e *UserNotFoundError) Error() string {
	return "user " + e.ID + " not found"
}

//goark:controller("adminController")
type AdminController struct{}

//goark:get("/admin/users/{id}")
func (c *AdminController) User(ctx *arkweb.Context) (map[string]string, error) {
	return nil, &UserNotFoundError{ID: ctx.PathValue("id")}
}

//goark:controller-advice("adminAdvice")
type AdminAdvice struct{}

//goark:exception-handler
func (a *AdminAdvice) NotFound(ctx *arkweb.Context, err *UserNotFoundError) arkweb.Result {
	return arkweb.JSON(http.StatusNotFound, map[string]string{"id": err.ID})
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	assertGeneratedSourceParses(t, generated)
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	expected := []string{
		"container.Register(registry, \"adminAdvice\"",
		"container.Register[goweb.Configurer](registry, \"adminAdvice.mvcAdviceConfigurer\"",
		"advice, err := container.GetByType[*AdminAdvice](ctx, resolver, " +
			"container.WithQualifier(\"adminAdvice\"))",
		"mvc.NewConfigurer().WithControllerAdvices(mvc.NewControllerAdvice(\"adminAdvice\"",
		"mvc.ExceptionHandlerAs[*UserNotFoundError](func(ctx *arkweb.Context, " +
			"err *UserNotFoundError) arkweb.Result",
		"return advice.NotFound(ctx, err)",
		"container.WithFactoryDependencies(\"adminAdvice\")",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc advice source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCExceptionHandlerReceiverIsNotAdvice_shouldReturnValidationError(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

import arkweb "goark.dev/arkarta/web"

type UserNotFoundError struct{}

func (e *UserNotFoundError) Error() string {
	return "not found"
}

type AdminAdvice struct{}

//goark:exception-handler
func (a *AdminAdvice) NotFound(err *UserNotFoundError) arkweb.Result {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil ||
		!strings.Contains(err.Error(), "requires mvc controller advice receiver type") {
		t.Fatalf("expected mvc advice receiver validation error, got %v", err)
	}
}

func TestGenerateAnnotations_whenMVCModelAttributePointerExists_shouldReturnValidationError(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

type UserSearchCriteria struct{}

//goark:controller("adminController")
type AdminController struct{}

//goark:get("/users/search")
//goark:model-attribute[criteria]
func (c *AdminController) Search(criteria *UserSearchCriteria) map[string]any {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil ||
		!strings.Contains(
			err.Error(),
			"model attribute parameter criteria must be a non-pointer struct value",
		) {
		t.Fatalf("expected model attribute pointer validation error, got %v", err)
	}
}

func TestGenerateAnnotations_whenMVCBodyAndModelAttributeCombined_shouldReturnError(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

type CreateUserRequest struct{}
type UserSearchCriteria struct{}

//goark:controller("adminController")
type AdminController struct{}

//goark:post("/users")
//goark:request-body[input]
//goark:model-attribute[criteria]
func (c *AdminController) Create(
	input CreateUserRequest,
	criteria UserSearchCriteria,
) map[string]any {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil ||
		!strings.Contains(
			err.Error(),
			"must not combine request body and model attribute parameters",
		) {
		t.Fatalf("expected request body and model attribute validation error, got %v", err)
	}
}

func TestGenerateAnnotations_whenMVCRequestParameterTypeUnsupported_shouldReturnValidationError(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

//goark:controller("adminController")
type AdminController struct{}

//goark:get("/users/{id}")
//goark:path-variable[id]("id")
func (c *AdminController) Detail(id int32) map[string]any {
	return map[string]any{"id": id}
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "unsupported mvc parameter type int32") {
		t.Fatalf("expected unsupported mvc parameter type error, got %v", err)
	}
}

func TestGenerateAnnotations_whenMVCRequestBodySelectorMissing_shouldReturnValidationError(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

//goark:controller
type AdminController struct{}

//goark:post("/admin/users")
//goark:request-body
func (c *AdminController) Create(input string) string {
	return input
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "requires parameter selector") {
		t.Fatalf("expected request body selector validation error, got %v", err)
	}
}

func TestGenerateAnnotations_whenPropertySourceHasName_shouldKeepLocationValue(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:configuration("admin")
//goark:property-source("config/app.properties", name="admin-config", ignoreResourceNotFound=true)
type AdminConfiguration struct{}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	assertGeneratedSourceParses(t, generated)
	text := string(generated)
	expected := `coreenv.LoadPropertiesPropertySource(ctx, loader, "config/app.properties", ` +
		`coreenv.WithPropertySourceName("admin-config"), ` +
		`coreenv.WithIgnoreResourceNotFound(true))`
	if !strings.Contains(text, expected) {
		t.Fatalf(
			"generated property source should use location value and source name separately:\n%s",
			text,
		)
	}
}

func TestGenerateAnnotations_whenMVCRouteReceiverIsNotController_shouldReturnValidationError(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

type AdminController struct{}

//goark:get("/admin/users")
func (c *AdminController) Users() []string {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "requires mvc controller receiver type") {
		t.Fatalf("expected mvc receiver validation error, got %v", err)
	}
}

func TestGenerateAnnotations_whenMVCContextUsesDifferentPackage_shouldReturnValidationError(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

import other "example.com/notark/web"

//goark:controller
type AdminController struct{}

//goark:get("/admin/users")
func (c *AdminController) Users(ctx *other.Context) []string {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "parameter must be *arkarta/web.Context") {
		t.Fatalf("expected mvc context validation error, got %v", err)
	}
}

func TestGenerateAnnotations_whenMVCReturnUsesDifferentResultType_shouldGenerateJSONValueHandler(
	t *testing.T,
) {
	dir := t.TempDir()
	notarkDir := filepath.Join(dir, "notark")
	if err := os.MkdirAll(notarkDir, 0o755); err != nil {
		t.Fatalf("create notark package failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(notarkDir, "result.go"), []byte(`package notark

type Result struct {
	Code int
}
`), 0o644); err != nil {
		t.Fatalf("write notark source failed: %v", err)
	}
	source := `package app

import "example.com/goark-generated-test/notark"

//goark:controller
type AdminController struct{}

//goark:get("/admin/users")
func (c *AdminController) Users() (notark.Result, error) {
	return notark.Result{Code: 1}, nil
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
	if !strings.Contains(text, "mvc.Return[any](200") {
		t.Fatalf("expected non-arkarta Result to use return value handler:\n%s", text)
	}
	if strings.Contains(text, "(arkweb.Result, error)") {
		t.Fatalf("non-arkarta Result must not generate arkweb result handler:\n%s", text)
	}
}
