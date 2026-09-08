package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenMVCRequestBodyValidatedExists_shouldGenerateBindJSONGroups(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

type CreateUserRequest struct {
	Name string ` + "`json:\"name\" arkarta:\"required\" arkarta-groups:\"create\"`" + `
}

//goark-web:rest-controller("userController")
type UserController struct{}

//goark-web:post("/users")
//goark-web:request-body[input]
//goark-web:validated("create")
func (c *UserController) Create(input CreateUserRequest) map[string]string {
	return map[string]string{"name": input.Name}
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
		`mvc.BindJSONGroups[CreateUserRequest, any](201`,
		`func(ctx *arkweb.Context, input CreateUserRequest) (any, error)`,
		`}, "create")`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf(
				"generated request body validation groups source missing %q:\n%s",
				fragment,
				text,
			)
		}
	}
}

func TestGenerateAnnotations_whenMVCRequestBodyValidatedEntityExists_shouldGenerateBindEntityGroups(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

import (
	"net/http"

	goweb "goark.dev/goark/web"
)

type CreateUserRequest struct {
	Name string ` + "`json:\"name\" arkarta:\"required\" arkarta-groups:\"create\"`" + `
}

type User struct {
	Name string ` + "`json:\"name\"`" + `
}

//goark-web:rest-controller("userController")
type UserController struct{}

//goark-web:post("/users")
//goark-web:request-body[input]
//goark-web:validated("create")
func (c *UserController) Create(input CreateUserRequest) (goweb.ResponseEntity[User], error) {
	return goweb.Status(http.StatusCreated, User{Name: input.Name}), nil
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
		`mvc.BindEntityGroups[CreateUserRequest, User](` +
			`func(ctx *arkweb.Context, input CreateUserRequest) ` +
			`(goweb.ResponseEntity[User], error)`,
		`return controller.Create(input)`,
		`}, "create")`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf(
				"generated request body response entity validation groups source missing %q:\n%s",
				fragment,
				text,
			)
		}
	}
}

func TestGenerateAnnotations_whenMVCMultipartBodyValidatedExists_shouldGenerateBindMultipartGroups(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

import servletmultipart "goark.dev/arkarta/servlet/multipart"

type UploadRequest struct {
	File servletmultipart.Part ` + "`form:\"file\" arkarta:\"required\" arkarta-groups:\"create\"`" + `
}

//goark-web:rest-controller("uploadController")
type UploadController struct{}

//goark-web:post("/uploads")
//goark-web:multipart-body[input]
//goark-web:validated(value="create")
func (c *UploadController) Upload(input UploadRequest) map[string]string {
	return map[string]string{"name": input.File.Name()}
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
	if !strings.Contains(text, `mvc.BindMultipartGroups[UploadRequest, any](201`) ||
		!strings.Contains(text, `}, []string{"create"})`) {
		t.Fatalf("generated multipart validation groups source is wrong:\n%s", text)
	}
}

func TestGenerateAnnotations_whenMVCValidatedModelAttributeExists_shouldGenerateGroups(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

type SearchCriteria struct {
	Name string ` + "`form:\"name\" arkarta:\"required\" arkarta-groups:\"search\"`" + `
}

//goark-web:controller("pageController")
type PageController struct{}

//goark-web:get("/users")
//goark-web:model-attribute[criteria]
//goark-web:validated("search")
func (c *PageController) Users(criteria SearchCriteria) string {
	return criteria.Name
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
	if !strings.Contains(
		text,
		`criteria, err := mvc.ModelAttributeGroups[SearchCriteria](ctx, "search")`,
	) {
		t.Fatalf("generated model attribute validation groups source is wrong:\n%s", text)
	}
}
