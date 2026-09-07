package clitest

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
		"container.Register(registry, \"userService\", NewUserService, " +
			"container.WithPrimary(), container.WithDependencies(\"userRepository\"))",
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
	if _, err := os.Stat(filepath.Join(root, "resource", "static", "index.html")); !os.IsNotExist(
		err,
	) {
		t.Fatalf("app scaffold should not generate web resources: %v", err)
	}
}

func TestCommand_whenNewWebProjectUsesExplicitModuleAndDirectory_shouldApplyFlagsBeforeName(
	t *testing.T,
) {
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
	if !strings.Contains(
		stdout.String(),
		"goark new [-type app|web] [-module <module-path>] [-dir <path>] <name>",
	) {
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

func TestCommand_whenGenerateConfigurationMissingRequiredFlags_shouldReturnUsageError(
	t *testing.T,
) {
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
	if !strings.Contains(
		stdout.String(),
		"goark codegen configuration --name <name> --package <package>",
	) {
		t.Fatalf("expected configuration help in stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}
