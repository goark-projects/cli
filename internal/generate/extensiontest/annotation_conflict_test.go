package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestAnnotationConflicts(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"service", "//goark:service\n//goark:service\ntype Service struct{}", "Service"},
		{"stereotype", "//goark:service\n//goark:repository\ntype Service struct{}", "Service"},
		{"scope", "//goark:service\n//goark:scope(\"singleton\")\n" +
			"//goark:scope(\"prototype\")\ntype Service struct{}", "scope"},
		{"trailing_type", "//goark:service\ntype Service struct{} //goark:service", "Service"},
		{"injection", "//goark:service\ntype Service struct {\n//goark:autowired\n" +
			"//goark:value(\"${name}\")\nName string\n}", "Service.Name"},
		{"trailing_field", "//goark:service\ntype Service struct {\n//goark:autowired\n" +
			"Name string //goark:autowired\n}", "Service.Name"},
		{"qualifier", "//goark:service\ntype Service struct {\n//goark:qualifier(\"a\")\n" +
			"//goark:named(\"b\")\nName string\n}", "Service.Name"},
		{"controller", "//goark-web:controller\n//goark-web:rest-controller\n" +
			"type Service struct{}", "Service"},
		{"const", "//goark:service\nconst Service = 1", "unsupported"},
		{"orphan", "//goark:service\n\ntype Service struct{}", "unattached"},
		{"orphan_injection", "type Service struct {\n//goark:autowired\nName string\n}",
			"Service"},
		{"field_selector", "//goark:service\ntype Service struct {\n" +
			"//goark:value[other](\"${name}\")\nName string\n}", "selector"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "app.go")
			if err := os.WriteFile(path, []byte("package app\n"+test.source+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			data, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
			if err == nil || !strings.Contains(err.Error(), test.want) || len(data) != 0 {
				t.Fatalf("want=%q, error=%v, output=%s", test.want, err, data)
			}
		})
	}
}

func TestTrailingInjectionIsGenerated(t *testing.T) {
	dir := t.TempDir()
	source := "package app\n//goark:service\ntype Service struct { " +
		"Name string //goark:value(\"${name}\")\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil || !strings.Contains(string(data), "${name}") {
		t.Fatalf("error=%v, output=%s", err, data)
	}
}

func TestExtensionRepeatabilityIsExplicit(t *testing.T) {
	for _, repeatable := range []bool{false, true} {
		dir := t.TempDir()
		source := "package app\n//goark-demo:mark\n//goark-demo:mark\ntype App struct{}\n"
		if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{
			Dir: dir,
			Extensions: []generate.AnnotationExtension{{
				Descriptors: []generate.AnnotationDescriptor{{
					Name: "goark-demo:mark", Repeatable: repeatable,
				}},
			}},
		})
		if (err == nil) != repeatable {
			t.Fatalf("repeatable=%t, error=%v", repeatable, err)
		}
	}
}

func TestBeanParameterAnnotationIdentity(t *testing.T) {
	for _, test := range []struct {
		selector string
		valid    bool
	}{
		{"b", true}, {"a", false}, {"param=\"a\"", false},
	} {
		dir := t.TempDir()
		source := `package app
//goark:configuration
//goark:property-source("a.properties")
//goark:property-source("b.properties")
type Config struct{}
//goark:bean
//goark:value[a]("${a}")
//goark:value[` + test.selector + `]("${b}")
func (Config) Text(a, b string) string { return a+b }
`
		if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
		if (err == nil) != test.valid {
			t.Fatalf("selector=%s, error=%v", test.selector, err)
		}
		if test.valid {
			for _, want := range []string{"${a}", "${b}", "a.properties", "b.properties"} {
				if !strings.Contains(string(out), want) {
					t.Fatalf("未生成 %s", want)
				}
			}
		}
	}
}
