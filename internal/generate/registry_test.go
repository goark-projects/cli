package generate_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/goark-projects/cli/internal/generate"
)

func TestGenerateRegistry_whenSpecHasConfigurations_shouldGenerateDeterministicRegistry(t *testing.T) {
	source, err := generate.GenerateRegistry(generate.RegistrySpec{
		PackageName:  "generated",
		FunctionName: "RegisterAdminConfigurations",
		Imports: []generate.ImportSpec{
			{Alias: "admincfg", Path: "example.com/app/internal/admin/config"},
		},
		Configurations: []generate.ConfigurationRegistrationSpec{
			{Type: "WebConfiguration"},
			{Type: "admincfg.AdminConfiguration"},
		},
	})
	if err != nil {
		t.Fatalf("generate registry failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "registry.go", source, parser.ParseComments); err != nil {
		t.Fatalf("generated registry should parse: %v\n%s", err, string(source))
	}

	text := string(source)
	expectedFragments := []string{
		"package generated",
		"\"github.com/goark-projects/goark\"",
		"admincfg \"example.com/app/internal/admin/config\"",
		"func RegisterAdminConfigurations(app *goark.ApplicationContext) error",
		"goark.RegisterConfiguration(app, admincfg.AdminConfiguration{})",
		"goark.RegisterConfiguration(app, WebConfiguration{})",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated registry missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateRegistry_whenFunctionNameMissing_shouldUseDefaultFunctionName(t *testing.T) {
	source, err := generate.GenerateRegistry(generate.RegistrySpec{
		PackageName: "generated",
		Configurations: []generate.ConfigurationRegistrationSpec{
			{Type: "AdminConfiguration"},
		},
	})
	if err != nil {
		t.Fatalf("generate registry failed: %v", err)
	}
	if !strings.Contains(string(source), "func RegisterConfigurations(app *goark.ApplicationContext) error") {
		t.Fatalf("expected default function name, got:\n%s", string(source))
	}
}

func TestGenerateRegistry_whenSpecInvalid_shouldReturnError(t *testing.T) {
	cases := []generate.RegistrySpec{
		{PackageName: "", Configurations: []generate.ConfigurationRegistrationSpec{{Type: "AdminConfiguration"}}},
		{PackageName: "bad-name", Configurations: []generate.ConfigurationRegistrationSpec{{Type: "AdminConfiguration"}}},
		{PackageName: "generated", FunctionName: "Register-Configurations", Configurations: []generate.ConfigurationRegistrationSpec{{Type: "AdminConfiguration"}}},
		{PackageName: "generated"},
		{PackageName: "generated", Configurations: []generate.ConfigurationRegistrationSpec{{Type: ""}}},
		{PackageName: "generated", Configurations: []generate.ConfigurationRegistrationSpec{{Type: "bad type"}}},
		{PackageName: "generated", Configurations: []generate.ConfigurationRegistrationSpec{{Type: "NewConfiguration()"}}},
		{PackageName: "generated", Configurations: []generate.ConfigurationRegistrationSpec{{Type: "adminConfiguration"}}},
		{PackageName: "generated", Configurations: []generate.ConfigurationRegistrationSpec{{Type: "cfg.adminConfiguration"}}},
		{PackageName: "generated", Imports: []generate.ImportSpec{{Path: "github.com/goark-projects/goark"}}, Configurations: []generate.ConfigurationRegistrationSpec{{Type: "AdminConfiguration"}}},
	}

	for _, item := range cases {
		if _, err := generate.GenerateRegistry(item); err == nil {
			t.Fatalf("expected invalid registry spec error for %#v", item)
		}
	}
}
