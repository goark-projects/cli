package generate_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenPackageHasGoarkAnnotations_shouldGenerateConfiguration(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type Database struct{}
type Repository struct{}

//goark:repository("repo")
//goark:primary
//goark:priority(10)
type UserRepository struct{}

//goark:service("userService")
//goark:lazy
//goark:depends-on("database")
//goark:order(5)
type UserService struct {
	//goark:autowired
	//goark:qualifier("repo")
	repository *UserRepository

	//goark:value("${feature.enabled:false}")
	enabled bool
}

//goark:configuration("app")
//goark:profile("prod")
//goark:property-source("file:app.properties")
type AppConfiguration struct{}

//goark:bean("database")
//goark:scope("singleton")
func (AppConfiguration) Database() (*Database, error) {
	return &Database{}, nil
}

//goark:bean("repository")
//goark:qualifier[database]("database")
func (AppConfiguration) Repository(database *Database) *Repository {
	return &Repository{}
}
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
		"package app",
		"func (AppConfiguration) ConfigureEnvironment(ctx context.Context, environment coreenv.ConfigurableEnvironment) error",
		"coreenv.LoadPropertiesPropertySource(ctx, loader, \"file:app.properties\")",
		"func (c AppConfiguration) RegisterWithContext(ctx context.Context, config goark.ConfigurationContext) error",
		"goark.ProfileCondition{Expression: \"(prod)\"}",
		"container.Register(registry, \"repo\"",
		"container.WithPrimary(), container.WithPriority(10)",
		"container.Register(registry, \"userService\"",
		"container.WithQualifier(\"repo\")",
		"container.WithInjectionDependencies(\"repo\")",
		"container.WithTypedDependencyInjector(func(ctx context.Context, resolver container.Resolver, out *UserService) error",
		"var err error",
		"goark.ResolveValueAs[bool](config.Environment(), \"${feature.enabled:false}\")",
		"container.WithLazy(), container.WithDependsOn(\"database\"), container.WithOrder(5)",
		"container.Register(registry, \"database\"",
		"container.Register(registry, \"repository\"",
		"container.GetByType[*Database](ctx, resolver, container.WithQualifier(\"database\"))",
		"container.WithFactoryDependencies(\"database\")",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenDependsOnHasMultipleValues_shouldGenerateAllManualDependencies(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type Repository struct{}

//goark:repository("repository")
type UserRepository struct{}

//goark:service("userService")
//goark:depends-on("database, cache")
//goark:depends-on("schemaMigrator", "redisClient")
type UserService struct {
	//goark:autowired
	repository *UserRepository
}
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
	text := string(generated)
	expected := []string{
		"container.WithDependsOn(\"database\", \"cache\", \"schemaMigrator\", \"redisClient\")",
		"container.WithInjectionDependencies(\"repository\")",
		"container.WithTypedDependencyInjector(func(ctx context.Context, resolver container.Resolver, out *UserService) error",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenBeanParameterUnannotated_shouldInferFactoryDependency(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type Database struct{}
type Repository struct{}

//goark:configuration("app")
type AppConfiguration struct{}

//goark:bean("database")
func (AppConfiguration) Database() *Database {
	return &Database{}
}

//goark:bean("repository")
func (AppConfiguration) Repository(database *Database) *Repository {
	return &Repository{}
}
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
	text := string(generated)
	expected := []string{
		"database, err = container.GetByType[*Database](ctx, resolver)",
		"container.WithFactoryDependencies(\"database\")",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenOptionalAutowiredProvided_shouldIgnoreOnlyMissingBean(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type Repository struct{}

//goark:service
type UserService struct {
	//goark:autowired(required=false)
	repository *Repository
}
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
	text := string(generated)
	expected := []string{
		"arkerrors \"goark.dev/goark/errors\"",
		"if !arkerrors.Is(err, arkerrors.CodeNotFound)",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated optional injection source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenBeanParameterUnnamed_shouldAllowSyntheticSelector(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type AppConfiguration struct{}

//goark:bean("port")
//goark:value[arg0]("8080")
func (AppConfiguration) Port(int) int {
	return 8080
}
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
	if !strings.Contains(string(generated), "arg0, err = goark.ResolveValueAs[int](config.Environment(), \"8080\")") {
		t.Fatalf("generated source should resolve synthetic arg0:\n%s", string(generated))
	}
}

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
		"mvc.GET(\"/admin/users\", mvc.JSON[any](200",
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

func TestGenerateAnnotations_whenMVCRequestContractsExist_shouldGenerateRouteOptionsAndParameterBindings(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import (
	servletmultipart "goark.dev/arkarta/servlet/multipart"
	arkweb "goark.dev/arkarta/web"
)

//goark:controller("adminController")
//goark:request-mapping("/admin")
type AdminController struct{}

//goark:post("/assets/{id}", consumes="multipart/form-data", produces="application/json", params="mode=fast,!debug", headers="X-Tenant=admin")
//goark:path-variable[id]("id")
//goark:matrix-variable[color]("color", required=false)
//goark:request-attribute[traceID]("traceID")
//goark:session-attribute[principal]("principal")
//goark:request-part[file]("file")
func (c *AdminController) Upload(ctx *arkweb.Context, id int64, color string, traceID string, principal string, file servletmultipart.Part) (map[string]any, error) {
	return map[string]any{
		"id": id,
		"color": color,
		"traceID": traceID,
		"principal": principal,
		"file": file.SubmittedFileName(),
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
		"mvc.POST(",
		"mvc.WithConsumes(\"multipart/form-data\")",
		"mvc.WithProduces(\"application/json\")",
		"mvc.WithParams(\"mode=fast\", \"!debug\")",
		"mvc.WithHeaders(\"X-Tenant=admin\")",
		"id, err := mvc.PathInt64(ctx, \"id\")",
		"color, err := mvc.MatrixVariableString(ctx, \"color\", mvc.WithRequired(false))",
		"traceID, err := mvc.RequestAttributeString(ctx, \"traceID\")",
		"principal, err := mvc.SessionAttributeString(ctx, \"principal\")",
		"file, err := mvc.RequestPart(ctx, \"file\")",
		"return controller.Upload(ctx, id, color, traceID, principal, file)",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc request contract source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCRequestBodyAndPathVariableExist_shouldGenerateBothBindings(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import arkweb "goark.dev/arkarta/web"

type UpdateUserRequest struct {
	Username string ` + "`json:\"username\"`" + `
}

type User struct {
	ID int64 ` + "`json:\"id\"`" + `
	Username string ` + "`json:\"username\"`" + `
}

//goark:controller("adminController")
//goark:request-mapping("/admin")
type AdminController struct{}

//goark:put("/users/{id}")
//goark:path-variable[id]("id")
//goark:request-body[input]
func (c *AdminController) Update(ctx *arkweb.Context, id int64, input UpdateUserRequest) (User, error) {
	return User{ID: id, Username: input.Username}, nil
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
		"mvc.PUT(\"/admin/users/{id}\", mvc.BindJSON[UpdateUserRequest, any](200",
		"id, err := mvc.PathInt64(ctx, \"id\")",
		"return controller.Update(ctx, id, input)",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc combined binding source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCModelAttributeExists_shouldGenerateAggregateBinding(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import arkweb "goark.dev/arkarta/web"

type UserSearchCriteria struct {
	Username string ` + "`form:\"username\" json:\"username\"`" + `
	Page int ` + "`form:\"page\" json:\"page\"`" + `
}

//goark:controller("adminController")
//goark:request-mapping("/admin")
type AdminController struct{}

//goark:get("/users/search")
//goark:model-attribute[criteria]
func (c *AdminController) Search(ctx *arkweb.Context, criteria UserSearchCriteria) (map[string]any, error) {
	return map[string]any{
		"username": criteria.Username,
		"page": criteria.Page,
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
		"criteria, err := mvc.ModelAttribute[UserSearchCriteria](ctx)",
		"return controller.Search(ctx, criteria)",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc model attribute source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCControllerAdviceExists_shouldGenerateExceptionHandlers(t *testing.T) {
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
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	expected := []string{
		"container.Register(registry, \"adminAdvice\"",
		"container.Register[goweb.Configurer](registry, \"adminAdvice.mvcAdviceConfigurer\"",
		"advice, err := container.GetByType[*AdminAdvice](ctx, resolver, container.WithQualifier(\"adminAdvice\"))",
		"mvc.NewConfigurer().WithExceptionHandlers(",
		"mvc.ExceptionHandlerAs[*UserNotFoundError](func(ctx *arkweb.Context, err *UserNotFoundError) arkweb.Result",
		"return advice.NotFound(ctx, err)",
		"container.WithFactoryDependencies(\"adminAdvice\")",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated mvc advice source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenMVCExceptionHandlerReceiverIsNotAdvice_shouldReturnValidationError(t *testing.T) {
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
	if err == nil || !strings.Contains(err.Error(), "requires mvc controller advice receiver type") {
		t.Fatalf("expected mvc advice receiver validation error, got %v", err)
	}
}

func TestGenerateAnnotations_whenMVCModelAttributePointerExists_shouldReturnValidationError(t *testing.T) {
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
	if err == nil || !strings.Contains(err.Error(), "model attribute parameter criteria must be a non-pointer struct value") {
		t.Fatalf("expected model attribute pointer validation error, got %v", err)
	}
}

func TestGenerateAnnotations_whenMVCRequestBodyAndModelAttributeCombined_shouldReturnValidationError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type CreateUserRequest struct{}
type UserSearchCriteria struct{}

//goark:controller("adminController")
type AdminController struct{}

//goark:post("/users")
//goark:request-body[input]
//goark:model-attribute[criteria]
func (c *AdminController) Create(input CreateUserRequest, criteria UserSearchCriteria) map[string]any {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "must not combine request body and model attribute parameters") {
		t.Fatalf("expected request body and model attribute validation error, got %v", err)
	}
}

func TestGenerateAnnotations_whenMVCRequestParameterTypeUnsupported_shouldReturnValidationError(t *testing.T) {
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

func TestGenerateAnnotations_whenMVCRequestBodySelectorMissing_shouldReturnValidationError(t *testing.T) {
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
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	text := string(generated)
	expected := `coreenv.LoadPropertiesPropertySource(ctx, loader, "config/app.properties", coreenv.WithPropertySourceName("admin-config"), coreenv.WithIgnoreResourceNotFound(true))`
	if !strings.Contains(text, expected) {
		t.Fatalf("generated property source should use location value and source name separately:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCRouteReceiverIsNotController_shouldReturnValidationError(t *testing.T) {
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

func TestGenerateAnnotations_whenMVCContextUsesDifferentPackage_shouldReturnValidationError(t *testing.T) {
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

func TestGenerateAnnotations_whenMVCReturnUsesDifferentResultType_shouldGenerateJSONValueHandler(t *testing.T) {
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
	if !strings.Contains(text, "mvc.JSON[any](200") {
		t.Fatalf("expected non-arkarta Result to use JSON value handler:\n%s", text)
	}
	if strings.Contains(text, "(arkweb.Result, error)") {
		t.Fatalf("non-arkarta Result must not generate arkweb result handler:\n%s", text)
	}
}

func TestGenerateAnnotations_whenUnknownAnnotationFound_shouldReturnError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:servcie
type UserService struct{}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), `unknown annotation "servcie"`) {
		t.Fatalf("expected unknown annotation error, got %v", err)
	}
}

func TestGenerateAnnotations_whenAnnotationHasTrailingContent_shouldReturnParseError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:service trailing
type UserService struct{}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), `annotation "service" has unsupported trailing content "trailing"`) {
		t.Fatalf("expected trailing content parse error, got %v", err)
	}
}

func TestGenerateAnnotations_whenCoreAnnotationMisplaced_shouldReturnValidationError(t *testing.T) {
	cases := []struct {
		name          string
		source        string
		errorFragment string
	}{
		{
			name: "service on interface",
			source: `package app

//goark:service
type UserService interface{}
`,
			errorFragment: `annotation "service" requires struct type target`,
		},
		{
			name: "bean on top level function",
			source: `package app

//goark:bean
func NewRepository() string { return "" }
`,
			errorFragment: `annotation "bean" requires concrete method with receiver`,
		},
		{
			name: "method qualifier without parameter selector",
			source: `package app

type AppConfiguration struct{}

//goark:bean
//goark:qualifier("repository")
func (AppConfiguration) Service(repository string) string { return repository }
`,
			errorFragment: `annotation "qualifier" on method target requires parameter selector`,
		},
	}

	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(item.source), 0o644); err != nil {
				t.Fatalf("write source failed: %v", err)
			}
			_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
			if err == nil || !strings.Contains(err.Error(), item.errorFragment) {
				t.Fatalf("expected validation error containing %q, got %v", item.errorFragment, err)
			}
		})
	}
}

func TestGenerateAnnotations_whenCoreAnnotationArgumentInvalid_shouldReturnValidationError(t *testing.T) {
	cases := []struct {
		name          string
		source        string
		errorFragment string
	}{
		{
			name: "empty argument",
			source: `package app

//goark:service("userService",)
type UserService struct{}
`,
			errorFragment: `annotation argument is empty`,
		},
		{
			name: "named type annotation with multiple values",
			source: `package app

//goark:service("userService", "ignored")
type UserService struct{}
`,
			errorFragment: `annotation "service" accepts at most one value argument`,
		},
		{
			name: "named type annotation with name and value",
			source: `package app

//goark:service(name="userService", "ignored")
type UserService struct{}
`,
			errorFragment: `annotation "service" accepts either name or value argument`,
		},
		{
			name: "single value annotation with multiple values",
			source: `package app

//goark:service
//goark:scope("singleton", "prototype")
type UserService struct{}
`,
			errorFragment: `annotation "scope" accepts exactly one value argument`,
		},
		{
			name: "named and positional value arguments",
			source: `package app

//goark:service
//goark:scope(value="singleton", "prototype")
type UserService struct{}
`,
			errorFragment: `duplicate annotation argument "value"`,
		},
		{
			name: "invalid order",
			source: `package app

//goark:service
//goark:order(foo)
type UserService struct{}
`,
			errorFragment: `annotation "order" requires integer value`,
		},
		{
			name: "unsupported scope",
			source: `package app

//goark:service
//goark:scope("request")
type UserService struct{}
`,
			errorFragment: `annotation "scope" has unsupported scope "request"`,
		},
		{
			name: "bean option on configuration",
			source: `package app

//goark:configuration
//goark:scope("singleton")
type AppConfiguration struct{}
`,
			errorFragment: `annotation "scope" requires component type target`,
		},
		{
			name: "missing selector parameter",
			source: `package app

type AppConfiguration struct{}

//goark:bean
//goark:qualifier[missing]("repository")
func (AppConfiguration) Service(repository string) string { return repository }
`,
			errorFragment: `annotation "qualifier" selector "missing" does not match any method parameter`,
		},
		{
			name: "invalid autowired required",
			source: `package app

type Repository struct{}

//goark:service
type UserService struct {
	//goark:autowired(required=maybe)
	repository *Repository
}
`,
			errorFragment: `annotation "autowired" argument "required" requires boolean value`,
		},
		{
			name: "property source on component",
			source: `package app

//goark:service
//goark:property-source("file:app.properties")
type UserService struct{}
`,
			errorFragment: `annotation "property-source" requires configuration type target`,
		},
		{
			name: "empty depends-on item",
			source: `package app

//goark:service
//goark:depends-on("database,")
type UserService struct{}
`,
			errorFragment: `annotation "depends-on" has empty dependency name`,
		},
	}

	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(item.source), 0o644); err != nil {
				t.Fatalf("write source failed: %v", err)
			}
			_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
			if err == nil || !strings.Contains(err.Error(), item.errorFragment) {
				t.Fatalf("expected validation error containing %q, got %v", item.errorFragment, err)
			}
		})
	}
}

func TestDefaultAnnotationOutputName_whenPackageNameEmpty_shouldUseFallback(t *testing.T) {
	if got := generate.DefaultAnnotationOutputName(""); got != "zz_goark_package_gen.go" {
		t.Fatalf("unexpected default output name: %q", got)
	}
}

func TestGenerateAnnotations_whenGroupedTypeAndGroupedFieldsHaveAnnotations_shouldGenerateRegistrations(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type Dependency struct{}

type (
	//goark:service
	GroupedService struct {
		//goark:resource
		db, cache *Dependency
	}
)
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
	text := string(generated)
	expected := []string{
		"container.Register(registry, \"groupedService\"",
		"container.WithQualifier(\"db\")",
		"container.WithQualifier(\"cache\")",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated grouped annotation source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenExtensionProvided_shouldBindAndGenerateWithoutCoreChanges(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:mapper
type UserMapper interface{}
`
	if err := os.WriteFile(filepath.Join(dir, "mapper.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{
		Dir: dir,
		Extensions: []generate.AnnotationExtension{
			{
				Descriptors: []generate.AnnotationDescriptor{
					{Name: "mapper", Targets: []generate.AnnotationTarget{generate.AnnotationTargetType}},
				},
				Binder:    mapperTestBinder{},
				Generator: mapperTestGenerator{},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	text := string(generated)
	if !strings.Contains(text, "const goarkMapperUserMapper = \"UserMapper\"") {
		t.Fatalf("generated extension source missing mapper constant:\n%s", text)
	}
	if !strings.Contains(text, "type GoarkPackageConfiguration struct{}") {
		t.Fatalf("generated source should keep default core configuration:\n%s", text)
	}
}

func TestGenerateAnnotations_whenExtensionUsesInterfaceMethod_shouldBindWithoutScannerChange(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import "context"

type User struct{}

//goark:mapper
type UserMapper interface {
	//goark:select("select * from users where id = ?")
	FindByID(ctx context.Context, id int64) (*User, error)
}
`
	if err := os.WriteFile(filepath.Join(dir, "mapper.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{
		Dir: dir,
		Extensions: []generate.AnnotationExtension{
			{
				Descriptors: []generate.AnnotationDescriptor{
					{Name: "mapper", Targets: []generate.AnnotationTarget{generate.AnnotationTargetType}},
					{Name: "select", Targets: []generate.AnnotationTarget{generate.AnnotationTargetMethod}},
				},
				Binder:    ormTestBinder{},
				Generator: ormTestGenerator{},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	text := string(generated)
	expected := []string{
		"const goarkORMMapperUserMapper = \"UserMapper\"",
		"const goarkORMSelectUserMapperFindByID = \"UserMapper.FindByID:select * from users where id = ?\"",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated orm extension source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenExtensionTargetInvalid_shouldReturnDescriptorError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:mapper
func NewMapper() {}
`
	if err := os.WriteFile(filepath.Join(dir, "mapper.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{
		Dir: dir,
		Extensions: []generate.AnnotationExtension{
			{
				Descriptors: []generate.AnnotationDescriptor{
					{Name: "mapper", Targets: []generate.AnnotationTarget{generate.AnnotationTargetType}},
				},
				Binder:    mapperTestBinder{},
				Generator: mapperTestGenerator{},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `annotation "mapper" does not support method target`) {
		t.Fatalf("expected descriptor target error, got %v", err)
	}
}

func assertGeneratedPackageBuilds(t *testing.T, dir string, generated []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "zz_goark_app_gen.go"), generated, 0o644); err != nil {
		t.Fatalf("write generated source failed: %v", err)
	}
	goarkRoot := filepath.ToSlash(filepath.Clean(filepath.Join(projectRoot(t), "..", "goark")))
	mod := fmt.Sprintf(`module example.com/goark-generated-test

go 1.25

require goark.dev/goark v0.0.0

replace goark.dev/goark => %s
`, goarkRoot)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatalf("write generated test module failed: %v", err)
	}
	cmd := exec.Command("go", "test", "-mod=mod", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated package should compile: %v\n%s", err, string(output))
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

type mapperTestBinder struct{}

func (mapperTestBinder) BindAnnotation(ctx *generate.AnnotationBindingContext, item generate.AnnotationItem) error {
	if !item.HasAnnotation("mapper") {
		return nil
	}
	value, _ := ctx.Value("test.mapper.types")
	types, _ := value.([]string)
	types = append(types, item.TypeName())
	ctx.SetValue("test.mapper.types", types)
	return nil
}

type mapperTestGenerator struct{}

func (mapperTestGenerator) GenerateAnnotation(ctx *generate.AnnotationGenerationContext) error {
	value, _ := ctx.Value("test.mapper.types")
	types, _ := value.([]string)
	sort.Strings(types)
	for _, typ := range types {
		ctx.WriteString("const goarkMapper" + typ + " = " + strconv.Quote(typ) + "\n\n")
	}
	return nil
}

type ormTestModel struct {
	mappers []string
	selects []string
}

type ormTestBinder struct{}

func (ormTestBinder) BindAnnotation(ctx *generate.AnnotationBindingContext, item generate.AnnotationItem) error {
	value, _ := ctx.Value("test.orm.model")
	model, _ := value.(*ormTestModel)
	if model == nil {
		model = &ormTestModel{}
		ctx.SetValue("test.orm.model", model)
	}
	if item.HasAnnotation("mapper") {
		model.mappers = append(model.mappers, item.TypeName())
	}
	if item.HasAnnotation("select") {
		names := item.Names()
		if len(names) == 0 {
			return nil
		}
		for _, annotation := range item.Annotations() {
			if annotation.Name == "select" {
				model.selects = append(model.selects, item.TypeName()+"."+names[0]+":"+annotation.Args["value"].Text())
			}
		}
	}
	return nil
}

type ormTestGenerator struct{}

func (ormTestGenerator) GenerateAnnotation(ctx *generate.AnnotationGenerationContext) error {
	value, _ := ctx.Value("test.orm.model")
	model, _ := value.(*ormTestModel)
	if model == nil {
		return nil
	}
	sort.Strings(model.mappers)
	sort.Strings(model.selects)
	for _, mapper := range model.mappers {
		ctx.WriteString("const goarkORMMapper" + mapper + " = " + strconv.Quote(mapper) + "\n\n")
	}
	for _, query := range model.selects {
		target, _, _ := strings.Cut(query, ":")
		name := strings.ReplaceAll(target, ".", "")
		ctx.WriteString("const goarkORMSelect" + name + " = " + strconv.Quote(query) + "\n\n")
	}
	return nil
}
