package generate_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/goark-projects/cli/internal/generate"
)

func TestGenerateConfiguration_whenSpecHasBeans_shouldGenerateDeterministicConfiguration(t *testing.T) {
	source, err := generate.GenerateConfiguration(generate.ConfigurationSpec{
		PackageName:       "generated",
		ConfigurationName: "user",
		TypeName:          "UserConfiguration",
		Order:             100,
		Imports: []generate.ImportSpec{
			{Alias: "svc", Path: "example.com/app/internal/service"},
		},
		Beans: []generate.BeanSpec{
			{
				Name:         "userService",
				Provider:     "svc.NewUserService",
				Dependencies: []string{"userRepository"},
				Primary:      true,
			},
			{
				Name:     "userRepository",
				Provider: "NewUserRepository",
				Lazy:     true,
			},
		},
	})
	if err != nil {
		t.Fatalf("generate configuration failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "user_configuration.go", source, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(source))
	}

	text := string(source)
	expectedFragments := []string{
		"package generated",
		"svc \"example.com/app/internal/service\"",
		"type UserConfiguration struct{}",
		"func (UserConfiguration) Name() string {\n\treturn \"user\"\n}",
		"func (UserConfiguration) Order() int {\n\treturn 100\n}",
		"container.Register(registry, \"userRepository\", NewUserRepository, container.WithLazy())",
		"container.Register(registry, \"userService\", svc.NewUserService, container.WithPrimary(), container.WithDependencies(\"userRepository\"))",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateConfiguration_whenTypeNameMissing_shouldDeriveExportedTypeName(t *testing.T) {
	source, err := generate.GenerateConfiguration(generate.ConfigurationSpec{
		PackageName:       "generated",
		ConfigurationName: "user-profile",
	})
	if err != nil {
		t.Fatalf("generate configuration failed: %v", err)
	}
	if !strings.Contains(string(source), "type UserProfileConfiguration struct{}") {
		t.Fatalf("expected derived type name, got:\n%s", string(source))
	}
}

func TestGenerateConfiguration_whenImportHasSpaces_shouldNormalizeImport(t *testing.T) {
	source, err := generate.GenerateConfiguration(generate.ConfigurationSpec{
		PackageName:       "generated",
		ConfigurationName: "user",
		Imports: []generate.ImportSpec{
			{Alias: " svc ", Path: " example.com/app/internal/service "},
		},
	})
	if err != nil {
		t.Fatalf("generate configuration failed: %v", err)
	}
	if !strings.Contains(string(source), "svc \"example.com/app/internal/service\"") {
		t.Fatalf("expected normalized import, got:\n%s", string(source))
	}
}

func TestGenerateConfiguration_whenSpecInvalid_shouldReturnError(t *testing.T) {
	cases := []generate.ConfigurationSpec{
		{PackageName: "", ConfigurationName: "user"},
		{PackageName: "bad-name", ConfigurationName: "user"},
		{PackageName: "generated", ConfigurationName: ""},
		{PackageName: "generated", ConfigurationName: "user", TypeName: "userConfiguration"},
		{PackageName: "generated", ConfigurationName: "user", Beans: []generate.BeanSpec{{Name: "repo"}}},
		{PackageName: "generated", ConfigurationName: "user", Beans: []generate.BeanSpec{{Name: "repo", Provider: "NewRepo", Scope: "request"}}},
	}

	for _, item := range cases {
		if _, err := generate.GenerateConfiguration(item); err == nil {
			t.Fatalf("expected invalid spec error for %#v", item)
		}
	}
}
