package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

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
