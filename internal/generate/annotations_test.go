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

func TestGenerateAnnotations_whenPackageHasGoarkAnnotations_shouldGenerateConfiguration(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type Database struct{}
type Repository struct{}

//goark:repository("repo")
//goark:primary
//goark:priority(10)
type UserRepository struct{}

//goark:service("userService")
//goark:lazy
//goark:depends-on("database")
//goark:order(5)
type UserService struct {
	//goark:autowired
	//goark:qualifier("repo")
	repository *UserRepository

	//goark:value("${feature.enabled:false}")
	enabled bool
}

//goark:configuration("app")
//goark:profile("prod")
//goark:property-source("file:app.properties")
type AppConfiguration struct{}

//goark:bean("database")
//goark:scope("singleton")
func (AppConfiguration) Database() (*Database, error) {
	return &Database{}, nil
}

//goark:bean("repository")
//goark:qualifier[database]("database")
func (AppConfiguration) Repository(database *Database) *Repository {
	return &Repository{}
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	expected := []string{
		"package app",
		"func (AppConfiguration) ConfigureEnvironment(ctx context.Context, environment coreenv.ConfigurableEnvironment) error",
		"coreenv.LoadPropertiesPropertySource(ctx, loader, \"file:app.properties\")",
		"func (c AppConfiguration) RegisterWithContext(ctx context.Context, config goark.ConfigurationContext) error",
		"goark.ProfileCondition{Expression: \"(prod)\"}",
		"container.Register(registry, \"repo\"",
		"container.WithPrimary(), container.WithPriority(10)",
		"container.Register(registry, \"userService\"",
		"container.WithQualifier(\"repo\")",
		"container.WithInjectionDependencies(\"repo\")",
		"container.WithTypedDependencyInjector(func(ctx context.Context, resolver container.Resolver, out *UserService) error",
		"var err error",
		"goark.ResolveValueAs[bool](config.Environment(), \"${feature.enabled:false}\")",
		"container.WithLazy(), container.WithDependsOn(\"database\"), container.WithOrder(5)",
		"container.Register(registry, \"database\"",
		"container.Register(registry, \"repository\"",
		"container.GetByType[*Database](ctx, resolver, container.WithQualifier(\"database\"))",
		"container.WithFactoryDependencies(\"database\")",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenSyntheticConfigurationCreated_shouldUseReservedName(t *testing.T) {
	dir := t.TempDir()
	source := `package admin

//goark:service("userService")
type UserService struct{}
`
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if !strings.Contains(string(generated), `return "goark.package.admin"`) {
		t.Fatalf("synthetic configuration should use reserved name:\n%s", generated)
	}
}

func TestGenerateAnnotations_whenDependsOnHasMultipleValues_shouldGenerateAllManualDependencies(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type Repository struct{}

//goark:repository("repository")
type UserRepository struct{}

//goark:service("userService")
//goark:depends-on("database, cache")
//goark:depends-on("schemaMigrator", "redisClient")
type UserService struct {
	//goark:autowired
	repository *UserRepository
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	text := string(generated)
	expected := []string{
		"container.WithDependsOn(\"database\", \"cache\", \"schemaMigrator\", \"redisClient\")",
		"container.WithInjectionDependencies(\"repository\")",
		"container.WithTypedDependencyInjector(func(ctx context.Context, resolver container.Resolver, out *UserService) error",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenBeanParameterUnannotated_shouldInferFactoryDependency(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type Database struct{}
type Repository struct{}

//goark:configuration("app")
type AppConfiguration struct{}

//goark:bean("database")
func (AppConfiguration) Database() *Database {
	return &Database{}
}

//goark:bean("repository")
func (AppConfiguration) Repository(database *Database) *Repository {
	return &Repository{}
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	text := string(generated)
	expected := []string{
		"database, err = container.GetByType[*Database](ctx, resolver)",
		"container.WithFactoryDependencies(\"database\")",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenOptionalAutowiredProvided_shouldIgnoreOnlyMissingBean(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type Repository struct{}

//goark:service
type UserService struct {
	//goark:autowired(required=false)
	repository *Repository
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	text := string(generated)
	expected := []string{
		"arkerrors \"goark.dev/goark/errors\"",
		"if !arkerrors.Is(err, arkerrors.CodeNotFound)",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated optional injection source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenBeanParameterUnnamed_shouldAllowSyntheticSelector(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type AppConfiguration struct{}

//goark:bean("port")
//goark:value[arg0]("8080")
func (AppConfiguration) Port(int) int {
	return 8080
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	if !strings.Contains(string(generated), "arg0, err = goark.ResolveValueAs[int](config.Environment(), \"8080\")") {
		t.Fatalf("generated source should resolve synthetic arg0:\n%s", string(generated))
	}
}
