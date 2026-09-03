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

func TestGenerateAnnotations_whenConfigurationPropertiesDeclared_shouldGenerateBinderMetadataAndRegistration(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import "time"

type TLSProperties struct {
	Enabled bool ` + "`goark:\",default=true\"`" + `
}

//goark:configuration-properties(prefix="server", ignoreUnknownFields=false)
type ServerProperties struct {
	Port int ` + "`goark:\",default=8080\"`" + `
	HTTPReadTimeout time.Duration ` + "`goark:\"read-timeout,required\"`" + `
	TLS *TLSProperties
}

func (p *ServerProperties) Validate() error { return nil }
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}
	writeConfigurationPropertiesRuntimeTest(t, dir)

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	text := string(generated)
	expected := []string{
		`func BindServerProperties(environment goark.Environment) (out *ServerProperties, err error)`,
		`coreenv.GetPropertyAsValue[time.Duration](environment, "server.read-timeout")`,
		`coreenv.GetPropertyAsValue[bool](environment, "server.tls.enabled")`,
		`goark.ValidateConfigurationPropertyNames(environment, "server"`,
		`any(out).(goark.ConfigurationPropertiesValidator)`,
		`func ServerPropertiesConfigurationMetadata() []goark.ConfigurationProperty`,
		`container.Register(registry, "serverProperties"`,
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated configuration properties source missing %q:\n%s", fragment, text)
		}
	}
	assertGeneratedPackageBuilds(t, dir, generated)
}

func TestGenerateAnnotations_whenConfigurationPropertiesUsesMap_shouldReturnExplicitError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:configuration-properties("logging.level")
type LoggingLevels struct {
	Levels map[string]string
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}
	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "map type, which is not supported yet") {
		t.Fatalf("GenerateAnnotations() error = %v, want map support boundary", err)
	}
}

func writeConfigurationPropertiesRuntimeTest(t *testing.T, dir string) {
	t.Helper()
	source := `package app

import (
	"testing"
	"time"

	coreenv "goark.dev/goark/core/env"
)

func TestGeneratedConfigurationPropertiesBinder(t *testing.T) {
	environment := coreenv.MustNewStandardEnvironment()
	source, err := coreenv.NewMapPropertySource("test", map[string]any{
		"server.read-timeout": "5s",
	})
	if err != nil { t.Fatal(err) }
	if err := environment.PropertySources().AddFirst(source); err != nil { t.Fatal(err) }
	properties, err := BindServerProperties(environment)
	if err != nil { t.Fatal(err) }
	if properties.Port != 8080 || properties.HTTPReadTimeout != 5*time.Second || properties.TLS == nil || !properties.TLS.Enabled {
		t.Fatalf("unexpected properties: %#v", properties)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "app_test.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write runtime test failed: %v", err)
	}
}
