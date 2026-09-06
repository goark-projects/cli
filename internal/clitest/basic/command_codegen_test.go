package clitest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestCommand_whenGenerateAnnotations_shouldWriteSplitGenFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "go.mod"),
		[]byte("module example.com/app\n\ngo 1.26.0\n"),
		0o644,
	); err != nil {
		t.Fatalf("write go.mod failed: %v", err)
	}
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
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", stdout.String())
	}
	output := filepath.Join(dir, "gen", "zz_goark_core_gen.go")
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read split annotation output failed: %v", err)
	}
	expected := []string{
		"package gen",
		`goarksource "example.com/app"`,
		"type GoarkPackageConfiguration struct{}",
		"container.Register(registry, \"userService\"",
	}
	for _, fragment := range expected {
		if !strings.Contains(string(data), fragment) {
			t.Fatalf("generated annotations missing %q:\n%s", fragment, data)
		}
	}
	if !strings.Contains(stderr.String(), "generated "+output) {
		t.Fatalf("expected generated path on stderr, got %q", stderr.String())
	}
	ignored, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil || string(ignored) != "**/gen/\n" {
		t.Fatalf("generated directory should be ignored: %v\n%s", err, ignored)
	}
}

func TestCommand_whenGenerateAnnotationsOutputFlagUsed_shouldRejectLegacyLayout(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Main([]string{
		"codegen", "annotations",
		"--output", "zz_goark_app_gen.go",
	}, &stdout, &stderr)

	if code != 2 || !strings.Contains(stderr.String(), "flag provided but not defined: -output") {
		t.Fatalf("legacy output flag should be rejected: code=%d stderr=%q", code, stderr.String())
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
