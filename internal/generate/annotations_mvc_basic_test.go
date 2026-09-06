package generate_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCControllerExists_shouldGenerateMVCConfiguration(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import arkweb "goark.dev/arkarta/web"

type User struct {
	ID int64
	Name string
}

//goark:service("userService")
type UserService struct{}

//goark:controller("adminController")
//goark:request-mapping("/admin")
type AdminController struct {
	//goark:autowired
	service *UserService
}

//goark:get("/users")
func (c *AdminController) Users(ctx *arkweb.Context) ([]User, error) {
	return []User{{ID: 1, Name: "root"}}, nil
}

//goark:delete("/users")
func (c *AdminController) Clear() {}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	expected := []string{
		"arkweb \"goark.dev/arkarta/web\"",
		"goweb \"goark.dev/goark/web\"",
		"\"goark.dev/goark/web/mvc\"",
		"type GoarkWebMVCConfiguration struct{}",
		"container.Register(registry, \"adminController\"",
		"container.WithTypedDependencyInjector(func(ctx context.Context, resolver container.Resolver, out *AdminController) error",
		"container.WithInjectionDependencies(\"userService\")",
		"container.Register[goweb.Configurer](registry, \"adminController.mvcConfigurer\"",
		"container.GetByType[*AdminController](ctx, resolver, container.WithQualifier(\"adminController\"))",
		"mvc.NewController(\"adminController\"",
		"mvc.GET(\"/admin/users\", mvc.Return[any](200",
		"return controller.Users(ctx)",
		"mvc.DELETE(\"/admin/users\", mvc.NoContent",
		"container.WithFactoryDependencies(\"adminController\")",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCRestControllerExists_shouldGenerateRestControllerConfiguration(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import arkweb "goark.dev/arkarta/web"

//goark:rest-controller("apiController")
//goark:request-mapping("/api")
type APIController struct{}

//goark:get("/status")
func (c *APIController) Status(ctx *arkweb.Context) (string, error) {
	return "UP", nil
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
	if !strings.Contains(text, "mvc.NewRestController(\"apiController\"") {
		t.Fatalf("generated mvc source should use NewRestController:\n%s", text)
	}
	if strings.Contains(text, "mvc.NewController(\"apiController\"") {
		t.Fatalf("generated mvc source must not downgrade rest controller:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCRequestBodyExists_shouldGenerateBindJSONHandler(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import arkweb "goark.dev/arkarta/web"

type CreateUserRequest struct {
	Username string ` + "`json:\"username\"`" + `
}

type User struct {
	Username string ` + "`json:\"username\"`" + `
}

//goark:controller("adminController")
//goark:request-mapping("/admin")
type AdminController struct{}

//goark:post("/users", status=201)
//goark:request-body[input]
func (c *AdminController) Create(ctx *arkweb.Context, input CreateUserRequest) (User, error) {
	return User{Username: input.Username}, nil
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
		"mvc.POST(\"/admin/users\", mvc.BindJSON[CreateUserRequest, any](201",
		"func(ctx *arkweb.Context, input CreateUserRequest) (any, error)",
		"return controller.Create(ctx, input)",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc request body source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCHeadAndOptionsRoutesExist_shouldGenerateMethodHelpers(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:controller("systemController")
//goark:request-mapping("/system")
type SystemController struct{}

//goark:head("/healthz")
func (c *SystemController) HeadHealth() error {
	return nil
}

//goark:request-mapping("/healthz", method="OPTIONS")
func (c *SystemController) OptionsHealth() {}
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
		"mvc.HEAD(\"/system/healthz\", mvc.NoContent",
		"return controller.HeadHealth()",
		"mvc.OPTIONS(\"/system/healthz\", mvc.NoContent",
		"controller.OptionsHealth()",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc head/options source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCRequestParametersExist_shouldGenerateParameterBindings(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import arkweb "goark.dev/arkarta/web"

//goark:controller("adminController")
//goark:request-mapping("/admin")
type AdminController struct{}

//goark:get("/users/{id}")
//goark:path-variable[id]("id")
//goark:request-param[query](name="q", defaultValue="all")
//goark:request-header[requestID]("X-Request-ID")
//goark:cookie-value[theme]("theme", required=false)
func (c *AdminController) Detail(ctx *arkweb.Context, id int64, query string, requestID string, theme string) (map[string]any, error) {
	return map[string]any{
		"id": id,
		"query": query,
		"requestID": requestID,
		"theme": theme,
	}, nil
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
		"id, err := mvc.PathInt64(ctx, \"id\")",
		"query, err := mvc.RequestParamString(ctx, \"q\", mvc.WithDefaultValue(\"all\"))",
		"requestID, err := mvc.RequestHeaderString(ctx, \"X-Request-ID\")",
		"theme, err := mvc.CookieValueString(ctx, \"theme\", mvc.WithRequired(false))",
		"return controller.Detail(ctx, id, query, requestID, theme)",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc parameter binding source missing %q:\n%s", fragment, text)
		}
	}
}
