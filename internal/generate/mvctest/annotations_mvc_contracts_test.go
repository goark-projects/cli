package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCRequestContractsExist_shouldGenerateRouteOptionsAndParameterBindings(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

import (
	servletmultipart "goark.dev/arkarta/servlet/multipart"
	arkweb "goark.dev/arkarta/web"
)

//goark:controller("adminController")
//goark:request-mapping("/admin")
type AdminController struct{}

` + `//goark:post("/assets/{id}", consumes="multipart/form-data", ` +
		`produces="application/json", params="mode=fast,!debug", ` +
		`headers="X-Tenant=admin")` + `
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

func TestGenerateAnnotations_whenMVCRequestBodyAndPathVariableExist_shouldGenerateBothBindings(
	t *testing.T,
) {
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

func TestGenerateAnnotations_whenMVCModelAttributeExists_shouldGenerateAggregateBinding(
	t *testing.T,
) {
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
