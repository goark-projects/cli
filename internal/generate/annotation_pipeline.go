package generate

import (
	"bytes"
	"go/ast"
	"go/token"
	"goark.dev/cli/internal/generate/annotationast"
	"goark.dev/cli/internal/genpipeline"
	"strings"

	"goark.dev/cli/internal/generate/annotationparse"
	"goark.dev/cli/internal/generate/annotationpolicy"
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
type AnnotationTarget = annotationast.Target

const (
	// AnnotationTargetType 表示类型声明注解。
	AnnotationTargetType AnnotationTarget = "type"
	// AnnotationTargetField 表示结构体字段注解。
	AnnotationTargetField AnnotationTarget = "field"
	// AnnotationTargetMethod 表示函数或方法注解。
	AnnotationTargetMethod AnnotationTarget = "method"
)

// AnnotationDescriptor 描述注解契约，领域名称包含 goark-web: 等前缀。
type AnnotationDescriptor = annotationpolicy.Descriptor[
	AnnotationTarget, AnnotationValidationContext,
]

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
	items       []AnnotationItem
	extensions  []AnnotationExtension
	descriptors map[string]AnnotationDescriptor
	namespaces  annotationparse.Namespaces
	spec        AnnotationScanSpec
}

// AnnotationItem 表示扫描得到的注解节点。
type AnnotationItem = annotationast.Item

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
	if err := pipeline.namespaces.ValidatePlacement(file, fset); err != nil {
		return err
	}
	for _, decl := range file.Decls {
		switch item := decl.(type) {
		case *ast.GenDecl:
			if item.Tok == token.TYPE {
				if err := scanTypeDeclaration(ctx, pipeline, fset, file, item); err != nil {
					return err
				}
			}
		case *ast.FuncDecl:
			annotations, err := pipeline.namespaces.ParseComments(item.Doc)
			if err != nil {
				return err
			}
			if len(annotations) == 0 {
				continue
			}
			annotationItem := annotationast.NewItem(annotationast.Info{
				Target:      AnnotationTargetMethod,
				PackageName: ctx.PackageName(),
				FileSet:     fset,
				File:        file,
				FuncDecl:    item,
				Annotations: annotations,
			})
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
	typeAnnotations, err := pipeline.namespaces.ParseComments(decl.Doc)
	if err != nil {
		return err
	}
	for _, spec := range decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		specAnnotations, err := pipeline.namespaces.ParseGroups(typeSpec.Doc, typeSpec.Comment)
		if err != nil {
			return err
		}
		annotations := mergeAnnotations(typeAnnotations, specAnnotations)
		if len(annotations) > 0 {
			item := annotationast.NewItem(annotationast.Info{
				Target:      AnnotationTargetType,
				PackageName: ctx.PackageName(),
				FileSet:     fset,
				File:        file,
				GenDecl:     decl,
				TypeSpec:    typeSpec,
				Annotations: annotations,
			})
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
		fieldAnnotations, err := pipeline.namespaces.ParseGroups(field.Doc, field.Comment)
		if err != nil {
			return err
		}
		if err := annotationpolicy.ValidateFieldOwner(fieldAnnotations, decl, typeSpec); err != nil {
			return err
		}
		if len(fieldAnnotations) == 0 {
			continue
		}
		item := annotationast.NewItem(annotationast.Info{
			Target:      AnnotationTargetField,
			PackageName: ctx.PackageName(),
			FileSet:     fset,
			File:        file,
			GenDecl:     decl,
			TypeSpec:    typeSpec,
			Field:       field,
			Annotations: fieldAnnotations,
		})
		if err := pipeline.dispatch(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// AnnotationPlan 保存完成语义绑定的包模型，渲染前不产生文件。
type AnnotationPlan struct {
	pkg      *annotationPackage
	values   map[string]any
	pipeline *annotationPipeline
}

// PrepareAnnotationPlans 完整扫描所有包后，依次校验、绑定并规划生成。
func PrepareAnnotationPlans(specs []AnnotationScanSpec) ([]*AnnotationPlan, error) {
	plans := make([]*AnnotationPlan, 0, len(specs))
	err := genpipeline.Execute(genpipeline.Step{Name: "scan", Run: func() error {
		for _, spec := range specs {
			pipeline, err := newAnnotationPipeline(spec)
			if err != nil {
				return err
			}
			pkg, values, err := scanAnnotations(spec, pipeline)
			if err != nil {
				return err
			}
			plans = append(plans, &AnnotationPlan{pkg, values, pipeline})
		}
		return nil
	}}, genpipeline.Step{Name: "validate", Run: func() error {
		for _, plan := range plans {
			p := plan.pipeline
			for _, item := range p.items {
				if err := p.validate(item); err != nil {
					return annotationparse.TargetError(item.FileSet(), item.TypeSpec(), item.Field(),
						item.FuncDecl(), item.ReceiverTypeName(), err)
				}
			}
		}
		return nil
	}}, genpipeline.Step{Name: "bind-and-plan", Run: func() error {
		for _, plan := range plans {
			ctx := &AnnotationBindingContext{spec: plan.pipeline.spec, pkg: plan.pkg, values: plan.values}
			if err := plan.pipeline.bind(ctx); err != nil {
				return err
			}
			if err := prepareExternalGeneration(plan.pkg, plan.values); err != nil {
				return err
			}
		}
		return nil
	}})
	if err != nil {
		return nil, err
	}
	return plans, nil
}
func (p *annotationPipeline) bind(ctx *AnnotationBindingContext) error {
	for _, item := range p.items {
		for _, extension := range p.extensions {
			if extension.Binder == nil || !itemMatchesDescriptors(item, extension.Descriptors) {
				continue
			}
			if err := extension.Binder.BindAnnotation(ctx, item); err != nil {
				return err
			}
		}
	}
	for _, extension := range p.extensions {
		if finalizer, ok := extension.Binder.(annotationBindingFinalizer); ok {
			if err := finalizer.FinalizeAnnotationBinding(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}
