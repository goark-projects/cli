package generate_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenGroupedTypeAndGroupedFieldsHaveAnnotations_shouldGenerateRegistrations(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

type Dependency struct{}

type (
	//goark:service
	GroupedService struct {
		//goark:resource
		db, cache *Dependency
	}
)
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
		"container.Register(registry, \"groupedService\"",
		"container.WithQualifier(\"db\")",
		"container.WithQualifier(\"cache\")",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated grouped annotation source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenExtensionProvided_shouldBindAndGenerateWithoutCoreChanges(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

//goark:mapper
type UserMapper interface{}
`
	if err := os.WriteFile(filepath.Join(dir, "mapper.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{
		Dir: dir,
		Extensions: []generate.AnnotationExtension{
			{
				Descriptors: []generate.AnnotationDescriptor{
					{
						Name:    "mapper",
						Targets: []generate.AnnotationTarget{generate.AnnotationTargetType},
					},
				},
				Binder:    mapperTestBinder{},
				Generator: mapperTestGenerator{},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	text := string(generated)
	if !strings.Contains(text, "const goarkMapperUserMapper = \"UserMapper\"") {
		t.Fatalf("generated extension source missing mapper constant:\n%s", text)
	}
	if !strings.Contains(text, "type GoarkPackageConfiguration struct{}") {
		t.Fatalf("generated source should keep default core configuration:\n%s", text)
	}
}

func TestGenerateAnnotations_whenExtensionUsesInterfaceMethod_shouldBindWithoutScannerChange(
	t *testing.T,
) {
	dir := t.TempDir()
	source := `package app

import "context"

type User struct{}

//goark:mapper
type UserMapper interface {
	//goark:select("select * from users where id = ?")
	FindByID(ctx context.Context, id int64) (*User, error)
}
`
	if err := os.WriteFile(filepath.Join(dir, "mapper.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{
		Dir: dir,
		Extensions: []generate.AnnotationExtension{
			{
				Descriptors: []generate.AnnotationDescriptor{
					{
						Name:    "mapper",
						Targets: []generate.AnnotationTarget{generate.AnnotationTargetType},
					},
					{
						Name:    "select",
						Targets: []generate.AnnotationTarget{generate.AnnotationTargetMethod},
					},
				},
				Binder:    interfaceMethodTestBinder{},
				Generator: interfaceMethodTestGenerator{},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_goark_app_gen.go", generated, parser.ParseComments); err != nil {
		t.Fatalf("generated source should parse: %v\n%s", err, string(generated))
	}
	text := string(generated)
	expected := []string{
		"const goarkExtensionMapperUserMapper = \"UserMapper\"",
		"const goarkExtensionSelectUserMapperFindByID = \"UserMapper.FindByID:select * from users where id = ?\"",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated interface method extension source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenExtensionTargetInvalid_shouldReturnDescriptorError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

//goark:mapper
func NewMapper() {}
`
	if err := os.WriteFile(filepath.Join(dir, "mapper.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{
		Dir: dir,
		Extensions: []generate.AnnotationExtension{
			{
				Descriptors: []generate.AnnotationDescriptor{
					{
						Name:    "mapper",
						Targets: []generate.AnnotationTarget{generate.AnnotationTargetType},
					},
				},
				Binder:    mapperTestBinder{},
				Generator: mapperTestGenerator{},
			},
		},
	})
	if err == nil ||
		!strings.Contains(err.Error(), `annotation "mapper" does not support method target`) {
		t.Fatalf("expected descriptor target error, got %v", err)
	}
}

type mapperTestBinder struct{}

func (mapperTestBinder) BindAnnotation(
	ctx *generate.AnnotationBindingContext,
	item generate.AnnotationItem,
) error {
	if !item.HasAnnotation("mapper") {
		return nil
	}
	value, _ := ctx.Value("test.mapper.types")
	types, _ := value.([]string)
	types = append(types, item.TypeName())
	ctx.SetValue("test.mapper.types", types)
	return nil
}

type mapperTestGenerator struct{}

func (mapperTestGenerator) GenerateAnnotation(ctx *generate.AnnotationGenerationContext) error {
	value, _ := ctx.Value("test.mapper.types")
	types, _ := value.([]string)
	sort.Strings(types)
	for _, typ := range types {
		ctx.WriteString("const goarkMapper" + typ + " = " + strconv.Quote(typ) + "\n\n")
	}
	return nil
}

type interfaceMethodTestModel struct {
	mappers []string
	selects []string
}

type interfaceMethodTestBinder struct{}

func (interfaceMethodTestBinder) BindAnnotation(
	ctx *generate.AnnotationBindingContext,
	item generate.AnnotationItem,
) error {
	value, _ := ctx.Value("test.interface-method.model")
	model, _ := value.(*interfaceMethodTestModel)
	if model == nil {
		model = &interfaceMethodTestModel{}
		ctx.SetValue("test.interface-method.model", model)
	}
	if item.HasAnnotation("mapper") {
		model.mappers = append(model.mappers, item.TypeName())
	}
	if item.HasAnnotation("select") {
		names := item.Names()
		if len(names) == 0 {
			return nil
		}
		for _, annotation := range item.Annotations() {
			if annotation.Name == "select" {
				model.selects = append(
					model.selects,
					item.TypeName()+"."+names[0]+":"+annotation.Args["value"].Text(),
				)
			}
		}
	}
	return nil
}

type interfaceMethodTestGenerator struct{}

func (interfaceMethodTestGenerator) GenerateAnnotation(
	ctx *generate.AnnotationGenerationContext,
) error {
	value, _ := ctx.Value("test.interface-method.model")
	model, _ := value.(*interfaceMethodTestModel)
	if model == nil {
		return nil
	}
	sort.Strings(model.mappers)
	sort.Strings(model.selects)
	for _, mapper := range model.mappers {
		ctx.WriteString(
			"const goarkExtensionMapper" + mapper + " = " + strconv.Quote(mapper) + "\n\n",
		)
	}
	for _, query := range model.selects {
		target, _, _ := strings.Cut(query, ":")
		name := strings.ReplaceAll(target, ".", "")
		ctx.WriteString("const goarkExtensionSelect" + name + " = " + strconv.Quote(query) + "\n\n")
	}
	return nil
}
