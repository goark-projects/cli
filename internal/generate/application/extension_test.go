package application_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
	"goark.dev/cli/internal/generate/application"
)

func TestExtension_whenWebApplicationDeclared_shouldGenerateBootLauncher(t *testing.T) {
	dir := writeSource(t, `package app

//goark:application(web=true)
//goark:configuration("app")
type Application struct{}

//goark:rest-controller("healthController")
type HealthController struct{}

//goark:get("/healthz")
func (*HealthController) Health() map[string]string {
	return map[string]string{"status": "UP"}
}
`)
	files, err := generate.GenerateAnnotationFiles(generate.AnnotationScanSpec{
		Dir: dir, SourceImportPath: "example.com/app/internal/app",
		Extensions: []generate.AnnotationExtension{application.Extension()},
	})
	if err != nil {
		t.Fatalf("生成应用代码失败: %v", err)
	}
	source := generatedFile(t, files, "zz_goark_application_gen.go")
	if _, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, 0); err != nil {
		t.Fatalf("生成代码无法解析: %v\n%s", err, source)
	}
	for _, fragment := range []string{
		"func Run(args []string) (exitCode int)",
		"boot.WithConfiguration(", "Application{}", "GoarkWebMVCConfiguration{}",
		"gbclog.AutoConfigure()", "gbcweb.AutoConfigure()", "<-ctx.Done()",
	} {
		if !strings.Contains(string(source), fragment) {
			t.Fatalf("生成应用代码缺少 %q:\n%s", fragment, source)
		}
	}
}

func TestExtension_whenApplicationIsNotConfiguration_shouldReject(t *testing.T) {
	dir := writeSource(t, "package app\n\n//goark:application\ntype Application struct{}\n")
	_, err := generate.GenerateAnnotationFiles(generate.AnnotationScanSpec{
		Dir: dir, SourceImportPath: "example.com/app/internal/app",
		Extensions: []generate.AnnotationExtension{application.Extension()},
	})
	if err == nil || !strings.Contains(err.Error(), "configuration") {
		t.Fatalf("错误 = %v, want application configuration validation", err)
	}
}

func TestExtension_whenNonWebApplicationDeclared_shouldGenerateOneShotLauncher(t *testing.T) {
	dir := writeSource(t, `package app

//goark:application
//goark:configuration("app")
type Application struct{}
`)
	files, err := generate.GenerateAnnotationFiles(generate.AnnotationScanSpec{
		Dir: dir, SourceImportPath: "example.com/app/internal/app",
		Extensions: []generate.AnnotationExtension{application.Extension()},
	})
	if err != nil {
		t.Fatalf("生成非 Web 应用代码失败: %v", err)
	}
	source := string(generatedFile(t, files, "zz_goark_application_gen.go"))
	for _, fragment := range []string{"func Run(args []string)", "slog.Info(\"应用已启动\")"} {
		if !strings.Contains(source, fragment) {
			t.Fatalf("生成非 Web 应用代码缺少 %q:\n%s", fragment, source)
		}
	}
	for _, fragment := range []string{"gbcweb", "signal.NotifyContext", "<-ctx.Done()"} {
		if strings.Contains(source, fragment) {
			t.Fatalf("生成非 Web 应用代码不应包含 %q:\n%s", fragment, source)
		}
	}
}

func TestExtension_whenWebArgumentInvalid_shouldReject(t *testing.T) {
	dir := writeSource(t, `package app

//goark:application(web=maybe)
//goark:configuration("app")
type Application struct{}
`)
	_, err := generate.GenerateAnnotationFiles(generate.AnnotationScanSpec{
		Dir: dir, Extensions: []generate.AnnotationExtension{application.Extension()},
	})
	if err == nil || !strings.Contains(err.Error(), "true or false") {
		t.Fatalf("错误 = %v, want invalid web argument", err)
	}
}
