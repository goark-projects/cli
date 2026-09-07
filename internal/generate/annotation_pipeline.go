package generate

import (
	"bytes"
	"go/ast"
	"go/token"
	"strings"
)

// AnnotationScanSpec 描述注解扫描生成输入。
type AnnotationScanSpec struct {
	Dir               string
	PackageName       string
	SourceImportPath  string
	GeneratorVersion  string
	ConfigurationName string
	TypeName          string
	Files             []string
	Extensions        []AnnotationExtension
}

// AnnotationTarget 表示注解所在的 Go 语法目标。
type AnnotationTarget string

const (
	// AnnotationTargetType 表示类型声明注解。
	AnnotationTargetType AnnotationTarget = "type"
	// AnnotationTargetField 表示结构体字段注解。
	AnnotationTargetField AnnotationTarget = "field"
	// AnnotationTargetMethod 表示函数或方法注解。
	AnnotationTargetMethod AnnotationTarget = "method"
)

// AnnotationDescriptor 描述一个可识别注解的名称、目标和验证逻辑。
type AnnotationDescriptor struct {
	Name     string
	Targets  []AnnotationTarget
	Validate func(AnnotationValidationContext) error
}

// AnnotationValidationContext 提供注解验证所需的上下文。
type AnnotationValidationContext struct {
	Target     AnnotationTarget
	Annotation Annotation
	Item       AnnotationItem
}

type annotationValidateFunc func(AnnotationValidationContext) error

func typeDesc(name string, validate annotationValidateFunc) AnnotationDescriptor {
	return newAnnotationDesc(name, validate, AnnotationTargetType)
}

func methodDesc(name string, validate annotationValidateFunc) AnnotationDescriptor {
	return newAnnotationDesc(name, validate, AnnotationTargetMethod)
}

// AnnotationBinder 将 AST 注解绑定到生成模型。
type AnnotationBinder interface {
	BindAnnotation(ctx *AnnotationBindingContext, item AnnotationItem) error
}

// AnnotationGenerator 将绑定后的模型写入生成源码。
type AnnotationGenerator interface {
	GenerateAnnotation(ctx *AnnotationGenerationContext) error
}

// AnnotationExtension 组合一个注解扩展的描述、绑定和生成阶段。
type AnnotationExtension struct {
	Name        string
	Descriptors []AnnotationDescriptor
	Binder      AnnotationBinder
	Generator   AnnotationGenerator
}

type annotationBindingFinalizer interface {
	FinalizeAnnotationBinding(ctx *AnnotationBindingContext) error
}

type annotationPipeline struct {
	extensions  []AnnotationExtension
	descriptors map[string]AnnotationDescriptor
	spec        AnnotationScanSpec
}

// AnnotationItem 表示扫描器发现的一处带 goark 注解的语法节点。
type AnnotationItem struct {
	target      AnnotationTarget
	packageName string
	fset        *token.FileSet
	file        *ast.File
	genDecl     *ast.GenDecl
	typeSpec    *ast.TypeSpec
	field       *ast.Field
	funcDecl    *ast.FuncDecl
	annotations []Annotation
}

// Target 返回当前注解所在语法目标。
func (i AnnotationItem) Target() AnnotationTarget { return i.target }

// PackageName 返回当前扫描包名。
func (i AnnotationItem) PackageName() string { return i.packageName }

// FileSet 返回当前扫描文件集。
func (i AnnotationItem) FileSet() *token.FileSet { return i.fset }

// File 返回当前 AST 文件。
func (i AnnotationItem) File() *ast.File { return i.file }

// GenDecl 返回当前通用声明，仅类型目标有效。
func (i AnnotationItem) GenDecl() *ast.GenDecl { return i.genDecl }

// TypeSpec 返回当前类型声明，仅类型或字段目标有效。
func (i AnnotationItem) TypeSpec() *ast.TypeSpec { return i.typeSpec }

// Field 返回当前字段声明，仅字段目标有效。
func (i AnnotationItem) Field() *ast.Field { return i.field }

// FuncDecl 返回当前函数声明，仅方法目标有效。
func (i AnnotationItem) FuncDecl() *ast.FuncDecl { return i.funcDecl }

// TypeName 返回当前类型名。
func (i AnnotationItem) TypeName() string {
	if i.typeSpec == nil {
		return ""
	}
	return i.typeSpec.Name.Name
}

// FuncName 返回当前函数名。
func (i AnnotationItem) FuncName() string {
	if i.funcDecl == nil {
		return ""
	}
	return i.funcDecl.Name.Name
}

// ReceiverTypeName 返回方法接收者类型名。
func (i AnnotationItem) ReceiverTypeName() string {
	if i.funcDecl == nil {
		return ""
	}
	return receiverTypeName(i.funcDecl.Recv)
}

// FieldNames 返回当前字段名列表。
func (i AnnotationItem) FieldNames() []string {
	if i.Target() != AnnotationTargetField {
		return nil
	}
	return i.Names()
}

// Names 返回当前语法目标声明的名称列表。
func (i AnnotationItem) Names() []string {
	if i.funcDecl != nil {
		return []string{i.funcDecl.Name.Name}
	}
	if i.field == nil {
		return nil
	}
	names := make([]string, 0, len(i.field.Names))
	for _, name := range i.field.Names {
		names = append(names, name.Name)
	}
	return names
}

// Annotations 返回当前节点上的 goark 注解副本。
func (i AnnotationItem) Annotations() []Annotation {
	out := make([]Annotation, len(i.annotations))
	copy(out, i.annotations)
	return out
}

// HasAnnotation 判断当前节点是否存在指定注解。
func (i AnnotationItem) HasAnnotation(name string) bool {
	return hasAnnotation(i.annotations, name)
}

// AnnotationBindingContext 持有扫描绑定阶段的共享状态。
type AnnotationBindingContext struct {
	spec   AnnotationScanSpec
	pkg    *annotationPackage
	values map[string]any
}

// PackageName 返回当前扫描包名。
func (c *AnnotationBindingContext) PackageName() string { return c.pkg.PackageName }

// Spec 返回注解扫描输入参数。
func (c *AnnotationBindingContext) Spec() AnnotationScanSpec { return c.spec }

// SetValue 写入扩展绑定阶段的共享模型。
func (c *AnnotationBindingContext) SetValue(key string, value any) { c.values[key] = value }

// Value 读取扩展绑定阶段的共享模型。
func (c *AnnotationBindingContext) Value(key string) (any, bool) {
	value, ok := c.values[key]
	return value, ok
}

// AnnotationGenerationContext 持有注解代码生成阶段的共享状态。
type AnnotationGenerationContext struct {
	pkg        *annotationPackage
	values     map[string]any
	body       bytes.Buffer
	imports    []ImportSpec
	importKeys map[string]struct{}
}

// PackageName 返回当前生成包名。
func (c *AnnotationGenerationContext) PackageName() string { return c.pkg.PackageName }

// SetValue 写入生成阶段共享状态。
func (c *AnnotationGenerationContext) SetValue(key string, value any) { c.values[key] = value }

// Value 读取绑定阶段写入的共享模型。
func (c *AnnotationGenerationContext) Value(key string) (any, bool) {
	value, ok := c.values[key]
	return value, ok
}

// AddImport 注册生成源码所需导入。
func (c *AnnotationGenerationContext) AddImport(alias string, path string) {
	alias = strings.TrimSpace(alias)
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	key := alias + "\x00" + path
	if _, exists := c.importKeys[key]; exists {
		return
	}
	c.importKeys[key] = struct{}{}
	c.imports = append(c.imports, ImportSpec{Alias: alias, Path: path})
}

// WriteString 写入生成源码正文。
func (c *AnnotationGenerationContext) WriteString(value string) {
	c.body.WriteString(value)
}

func scanAnnotationFile(
	ctx *AnnotationBindingContext, pipeline *annotationPipeline,
	fset *token.FileSet, file *ast.File,
) error {
	for _, decl := range file.Decls {
		switch item := decl.(type) {
		case *ast.GenDecl:
			if item.Tok == token.TYPE {
				if err := scanTypeDeclaration(ctx, pipeline, fset, file, item); err != nil {
					return err
				}
			}
		case *ast.FuncDecl:
			annotations, err := parseAnnotations(item.Doc)
			if err != nil {
				return err
			}
			if len(annotations) == 0 {
				continue
			}
			annotationItem := AnnotationItem{
				target:      AnnotationTargetMethod,
				packageName: ctx.PackageName(),
				fset:        fset,
				file:        file,
				funcDecl:    item,
				annotations: annotations,
			}
			if err := pipeline.dispatch(ctx, annotationItem); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanTypeDeclaration(
	ctx *AnnotationBindingContext, pipeline *annotationPipeline,
	fset *token.FileSet, file *ast.File, decl *ast.GenDecl,
) error {
	typeAnnotations, err := parseAnnotations(decl.Doc)
	if err != nil {
		return err
	}
	for _, spec := range decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		specAnnotations, err := parseAnnotations(typeSpec.Doc)
		if err != nil {
			return err
		}
		annotations := mergeAnnotations(typeAnnotations, specAnnotations)
		if len(annotations) > 0 {
			item := AnnotationItem{
				target:      AnnotationTargetType,
				packageName: ctx.PackageName(),
				fset:        fset,
				file:        file,
				genDecl:     decl,
				typeSpec:    typeSpec,
				annotations: annotations,
			}
			if err := pipeline.dispatch(ctx, item); err != nil {
				return err
			}
		}
		structType, ok := typeSpec.Type.(*ast.StructType)
		if ok {
			if err := scanStructFields(ctx, pipeline, fset, file, decl, typeSpec, structType); err != nil {
				return err
			}
		}
		interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
		if ok {
			if err := scanInterfaceMethods(
				ctx, pipeline, fset, file, decl, typeSpec, interfaceType,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanStructFields(
	ctx *AnnotationBindingContext, pipeline *annotationPipeline,
	fset *token.FileSet, file *ast.File, decl *ast.GenDecl,
	typeSpec *ast.TypeSpec, structType *ast.StructType,
) error {
	for _, field := range structType.Fields.List {
		fieldAnnotations, err := parseAnnotations(field.Doc)
		if err != nil {
			return err
		}
		if len(fieldAnnotations) == 0 {
			continue
		}
		item := AnnotationItem{
			target:      AnnotationTargetField,
			packageName: ctx.PackageName(),
			fset:        fset,
			file:        file,
			genDecl:     decl,
			typeSpec:    typeSpec,
			field:       field,
			annotations: fieldAnnotations,
		}
		if err := pipeline.dispatch(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
