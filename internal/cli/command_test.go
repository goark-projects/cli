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
		"generate", "configuration",
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

func TestCommand_whenGenerateConfigurationToFile_shouldWriteFileAndReportToStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	output := filepath.Join(t.TempDir(), "internal", "generated", "user_configuration.go")

	code := Main([]string{
		"gen", "configuration",
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

	code := Main([]string{"generate", "configuration", "--name", "user"}, &stdout, &stderr)

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

	code := Main([]string{"generate", "configuration", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "goark generate configuration --name <name> --package <package>") {
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
		"generate", "registry",
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
		"gen", "registry",
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

	code := Main([]string{"generate", "registry", "--package", "generated"}, &stdout, &stderr)

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

	code := Main([]string{"generate", "registry", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "goark generate registry --package <package> --configuration <type>") {
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
		"generate", "annotations",
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
		"gen", "annotations",
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

	code := Main([]string{"generate", "annotations", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "goark generate annotations --dir <package-dir>") {
		t.Fatalf("expected annotations help in stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestCommand_whenGenerateORMToStdout_shouldPrintGeneratedSource(t *testing.T) {
	dir := writeORMFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{
		"generate", "orm",
		"--dir", dir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	expected := []string{
		"package sample",
		"func RegisterGoarkORMMetadata(registry *orm.Registry) error",
		"Namespace: \"system.user.UserMapper\"",
		"func NewUserMapper(session orm.Session) UserMapper",
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated ORM output missing %q:\n%s", fragment, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestCommand_whenGenerateORMToFile_shouldWriteFileAndReportToStderr(t *testing.T) {
	dir := writeORMFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	output := filepath.Join(dir, "zz_goark_orm_sample_gen.go")

	code := Main([]string{
		"gen", "orm",
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
		t.Fatalf("read generated ORM file failed: %v", err)
	}
	if !strings.Contains(string(data), "type goarkORMUserMapper struct") {
		t.Fatalf("unexpected generated ORM file:\n%s", string(data))
	}
}

func TestCommand_whenGenerateORMHelpRequested_shouldReturnSuccess(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Main([]string{"generate", "orm", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "goark generate orm [pattern] [flags]") {
		t.Fatalf("expected ORM help in stdout, got %q", stdout.String())
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

func writeORMFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mapper.go"), []byte(`package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true;auto-increment=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(sql="select id, name from sys_user where id = #{id}")
	FindByID(ctx context.Context, id int64) (*User, error)
}
`), 0o644); err != nil {
		t.Fatalf("write ORM source failed: %v", err)
	}
	return dir
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
