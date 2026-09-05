package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommand_whenGenerateConfigurationToStdout_shouldPrintGeneratedSource(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{
		"codegen", "configuration",
		"--name", "user",
		"--package", "generated",
		"--type", "UserConfiguration",
		"--order", "100",
		"--bean", "userRepository=NewUserRepository;lazy",
		"--bean", "userService=NewUserService;deps=userRepository;primary",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	expected := []string{
		"package generated",
		"type UserConfiguration struct{}",
		"return \"user\"",
		"return 100",
		"container.Register(registry, \"userRepository\", NewUserRepository, container.WithLazy())",
		"container.Register(registry, \"userService\", NewUserService, container.WithPrimary(), container.WithDependencies(\"userRepository\"))",
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated output missing %q:\n%s", fragment, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestCommand_whenNewWebAppRequested_shouldWriteSkeleton(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dir := filepath.Join(t.TempDir(), "admin")

	code := Main([]string{
		"new",
		"-type", "web",
		"-module", "example.com/admin",
		"-dir", dir,
		"admin",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "created "+dir) {
		t.Fatalf("expected created path on stderr, got %q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "cmd", "server", "main.go")); err != nil {
		t.Fatalf("expected generated main.go: %v", err)
	}
}

func TestCommand_whenNewProjectNameProvided_shouldUseSimpleDefaults(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := t.TempDir()

	code := (Command{Dir: root, Out: &stdout, Err: &stderr}).Run([]string{"new", "ac"})

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	assertGeneratedFileContains(t, filepath.Join(root, "go.mod"), "module ac")
	assertGeneratedFileContains(t, filepath.Join(root, "goark.build"), "name = \"ac\"")
	assertGeneratedFileContains(t, filepath.Join(root, "goark.build"), "main = \"./cmd/app\"")
	assertGeneratedFileContains(t, filepath.Join(root, "goark.build"), "output = \"./build/ac\"")
	if _, err := os.Stat(filepath.Join(root, "resource", "static", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("app scaffold should not generate web resources: %v", err)
	}
}

func TestCommand_whenNewWebProjectUsesExplicitModuleAndDirectory_shouldApplyFlagsBeforeName(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dir := filepath.Join(t.TempDir(), "ac")

	code := Main([]string{
		"new", "-type", "web",
		"-module", "github.com/ac/aaa",
		"-dir", dir,
		"ac",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	assertGeneratedFileContains(t, filepath.Join(dir, "go.mod"), "module github.com/ac/aaa")
	assertGeneratedFileContains(t, filepath.Join(dir, "goark.build"), "name = \"ac\"")
}

func TestCommand_whenLegacyNewAppSyntaxUsed_shouldReturnUsageError(t *testing.T) {
	var stderr bytes.Buffer

	code := Main([]string{"new", "app", "--module", "example.com/legacy"}, &bytes.Buffer{}, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestCommand_whenLegacyNewAppHelpTopicUsed_shouldReturnUsageError(t *testing.T) {
	var stderr bytes.Buffer

	code := Main([]string{"help", "new", "app"}, &bytes.Buffer{}, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestCommand_whenNewProjectTypeUnsupported_shouldReturnUsageError(t *testing.T) {
	var stderr bytes.Buffer

	code := Main([]string{"new", "-type", "desktop", "ac"}, &bytes.Buffer{}, &stderr)

	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), `不支持的项目类型 "desktop"`) {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestCommand_whenNewAppHelpRequested_shouldReturnSuccess(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{"new", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "goark new [-type app|web] [-module <module-path>] [-dir <path>] <name>") {
		t.Fatalf("expected new app help in stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func assertGeneratedFileContains(t *testing.T, path string, fragment string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file %s failed: %v", path, err)
	}
	if !strings.Contains(string(data), fragment) {
		t.Fatalf("generated file %s missing %q:\n%s", path, fragment, data)
	}
}

func TestCommand_whenGenerateConfigurationToFile_shouldWriteFileAndReportToStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	output := filepath.Join(t.TempDir(), "internal", "generated", "user_configuration.go")

	code := Main([]string{
		"codegen", "configuration",
		"--name", "user",
		"--package", "generated",
		"--output", output,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "generated "+output) {
		t.Fatalf("expected generated path on stderr, got %q", stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read generated file failed: %v", err)
	}
	if !strings.Contains(string(data), "type UserConfiguration struct{}") {
		t.Fatalf("unexpected generated file:\n%s", string(data))
	}
}

func TestCommand_whenGenerateConfigurationMissingRequiredFlags_shouldReturnUsageError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{"codegen", "configuration", "--name", "user"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected usage exit code 2, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--package string") {
		t.Fatalf("expected configuration help in stderr, got %q", stderr.String())
	}
}

func TestCommand_whenGenerateConfigurationHelpRequested_shouldReturnSuccess(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{"codegen", "configuration", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "goark codegen configuration --name <name> --package <package>") {
		t.Fatalf("expected configuration help in stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestCommand_whenGenerateRegistryToStdout_shouldPrintGeneratedSource(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{
		"codegen", "registry",
		"--package", "generated",
		"--function", "RegisterAdminConfigurations",
		"--configuration", "UserConfiguration",
		"--configuration", "HTTPConfiguration",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	expected := []string{
		"package generated",
		"\"goark.dev/goark\"",
		"func RegisterAdminConfigurations(app *goark.ApplicationContext) error",
		"goark.RegisterConfiguration(app, HTTPConfiguration{})",
		"goark.RegisterConfiguration(app, UserConfiguration{})",
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated registry missing %q:\n%s", fragment, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestCommand_whenGenerateRegistryToFile_shouldWriteFileAndReportToStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	output := filepath.Join(t.TempDir(), "internal", "generated", "registry.go")

	code := Main([]string{
		"codegen", "registry",
		"--package", "generated",
		"--configuration", "AdminConfiguration",
		"--output", output,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "generated "+output) {
		t.Fatalf("expected generated path on stderr, got %q", stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read generated registry failed: %v", err)
	}
	if !strings.Contains(string(data), "func RegisterConfigurations(app *goark.ApplicationContext) error") {
		t.Fatalf("unexpected generated registry:\n%s", string(data))
	}
}

func TestCommand_whenGenerateRegistryMissingRequiredFlags_shouldReturnUsageError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{"codegen", "registry", "--package", "generated"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected usage exit code 2, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--configuration value") {
		t.Fatalf("expected registry help in stderr, got %q", stderr.String())
	}
}

func TestCommand_whenGenerateRegistryHelpRequested_shouldReturnSuccess(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{"codegen", "registry", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "goark codegen registry --package <package> --configuration <type>") {
		t.Fatalf("expected registry help in stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestCommand_whenGenerateAnnotationsToStdout_shouldPrintGeneratedSource(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(`package app

//goark:service
type UserService struct{}
`), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{
		"codegen", "annotations",
		"--dir", dir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	expected := []string{
		"package app",
		"type GoarkPackageConfiguration struct{}",
		"container.Register(registry, \"userService\"",
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated annotations missing %q:\n%s", fragment, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestCommand_whenGenerateAnnotationsToFile_shouldWriteFileAndReportToStderr(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(`package app

//goark:service
type UserService struct{}
`), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	output := filepath.Join(dir, "zz_goark_app_gen.go")

	code := Main([]string{
		"codegen", "annotations",
		"--dir", dir,
		"--output", output,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "generated "+output) {
		t.Fatalf("expected generated path on stderr, got %q", stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read generated annotations failed: %v", err)
	}
	if !strings.Contains(string(data), "type GoarkPackageConfiguration struct{}") {
		t.Fatalf("unexpected generated annotations:\n%s", string(data))
	}
}

func TestCommand_whenGenerateAnnotationsHelpRequested_shouldReturnSuccess(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{"codegen", "annotations", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "goark codegen annotations --dir <package-dir>") {
		t.Fatalf("expected annotations help in stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestParseBeanSpec_whenSpecHasOptions_shouldParseBean(t *testing.T) {
	bean, err := parseBeanSpec("userService=NewUserService;deps=userRepository, clock;scope=prototype;lazy;primary")
	if err != nil {
		t.Fatalf("parse bean failed: %v", err)
	}
	if bean.Name != "userService" || bean.Provider != "NewUserService" {
		t.Fatalf("unexpected bean identity: %#v", bean)
	}
	if bean.Scope != "prototype" || !bean.Lazy || !bean.Primary {
		t.Fatalf("unexpected bean options: %#v", bean)
	}
	if len(bean.Dependencies) != 2 || bean.Dependencies[0] != "userRepository" || bean.Dependencies[1] != "clock" {
		t.Fatalf("unexpected dependencies: %#v", bean.Dependencies)
	}
}

func TestParseBeanSpec_whenSpecInvalid_shouldReturnError(t *testing.T) {
	cases := []string{
		"",
		"userService",
		"userService=NewUserService;unknown",
		"userService=NewUserService;deps=",
	}
	for _, item := range cases {
		if _, err := parseBeanSpec(item); err == nil {
			t.Fatalf("expected parse error for %q", item)
		}
	}
}
