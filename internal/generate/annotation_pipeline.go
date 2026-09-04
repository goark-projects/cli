package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// AnnotationScanSpec 描述注解扫描生成输入。
type AnnotationScanSpec struct {
	Dir               string
	PackageName       string
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
func (i AnnotationItem) Target() AnnotationTarget {
	return i.target
}

// PackageName 返回当前扫描包名。
func (i AnnotationItem) PackageName() string {
	return i.packageName
}

// FileSet 返回当前扫描文件集。
func (i AnnotationItem) FileSet() *token.FileSet {
	return i.fset
}

// File 返回当前 AST 文件。
func (i AnnotationItem) File() *ast.File {
	return i.file
}

// GenDecl 返回当前通用声明，仅类型目标有效。
func (i AnnotationItem) GenDecl() *ast.GenDecl {
	return i.genDecl
}

// TypeSpec 返回当前类型声明，仅类型或字段目标有效。
func (i AnnotationItem) TypeSpec() *ast.TypeSpec {
	return i.typeSpec
}

// Field 返回当前字段声明，仅字段目标有效。
func (i AnnotationItem) Field() *ast.Field {
	return i.field
}

// FuncDecl 返回当前函数声明，仅方法目标有效。
func (i AnnotationItem) FuncDecl() *ast.FuncDecl {
	return i.funcDecl
}

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
func (c *AnnotationBindingContext) PackageName() string {
	return c.pkg.PackageName
}

// Spec 返回注解扫描输入参数。
func (c *AnnotationBindingContext) Spec() AnnotationScanSpec {
	return c.spec
}

// SetValue 写入扩展绑定阶段的共享模型。
func (c *AnnotationBindingContext) SetValue(key string, value any) {
	c.values[key] = value
}

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
func (c *AnnotationGenerationContext) PackageName() string {
	return c.pkg.PackageName
}

// SetValue 写入生成阶段共享状态。
func (c *AnnotationGenerationContext) SetValue(key string, value any) {
	c.values[key] = value
}

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

func (c *AnnotationGenerationContext) buffer() *bytes.Buffer {
	return &c.body
}

type annotationPackage struct {
	PackageName string
	fset        *token.FileSet
	types       map[string]annotationTypeDeclaration
}

type annotationTypeDeclaration struct {
	file *ast.File
	spec *ast.TypeSpec
}

// GenerateAnnotations 扫描 Go 源码注解并生成 goark 注册代码。
func GenerateAnnotations(spec AnnotationScanSpec) ([]byte, error) {
	pipeline, err := newAnnotationPipeline(spec.Extensions)
	if err != nil {
		return nil, err
	}
	pkg, values, err := scanAnnotations(spec, pipeline)
	if err != nil {
		return nil, err
	}
	return renderAnnotationPackage(pkg, values, pipeline)
}

func newAnnotationPipeline(extensions []AnnotationExtension) (*annotationPipeline, error) {
	all := append(defaultAnnotationExtensions(), extensions...)
	pipeline := &annotationPipeline{
		extensions:  all,
		descriptors: make(map[string]AnnotationDescriptor),
	}
	for _, extension := range all {
		for _, descriptor := range extension.Descriptors {
			name := strings.TrimSpace(descriptor.Name)
			if name == "" {
				return nil, fmt.Errorf("annotation descriptor name is required")
			}
			if _, exists := pipeline.descriptors[name]; exists {
				return nil, fmt.Errorf("duplicate annotation descriptor %q", name)
			}
			descriptor.Name = name
			pipeline.descriptors[name] = descriptor
		}
	}
	return pipeline, nil
}

func scanAnnotations(spec AnnotationScanSpec, pipeline *annotationPipeline) (*annotationPackage, map[string]any, error) {
	dir := strings.TrimSpace(spec.Dir)
	if dir == "" {
		dir = "."
	}
	fset := token.NewFileSet()
	packages, err := parseAnnotationPackages(fset, dir, spec.Files)
	if err != nil {
		return nil, nil, err
	}
	if len(packages) == 0 {
		return nil, nil, fmt.Errorf("no Go package found in %s", dir)
	}
	packageNames := make([]string, 0, len(packages))
	for name := range packages {
		packageNames = append(packageNames, name)
	}
	sort.Strings(packageNames)
	packageName := strings.TrimSpace(spec.PackageName)
	if packageName == "" {
		if len(packageNames) != 1 {
			return nil, nil, fmt.Errorf("multiple Go packages found in %s: %s", dir, strings.Join(packageNames, ", "))
		}
		packageName = packageNames[0]
	}
	parsedPackage := packages[packageName]
	if parsedPackage == nil {
		return nil, nil, fmt.Errorf("package %q not found in %s", packageName, dir)
	}

	pkg := &annotationPackage{PackageName: packageName, fset: fset, types: make(map[string]annotationTypeDeclaration)}
	for _, file := range parsedPackage.Files {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, item := range general.Specs {
				typeSpec, ok := item.(*ast.TypeSpec)
				if ok {
					pkg.types[typeSpec.Name.Name] = annotationTypeDeclaration{file: file, spec: typeSpec}
				}
			}
		}
	}
	ctx := &AnnotationBindingContext{
		spec:   spec,
		pkg:    pkg,
		values: make(map[string]any),
	}
	files := sortedPackageFiles(fset, parsedPackage)
	for _, file := range files {
		if err := scanAnnotationFile(ctx, pipeline, fset, file); err != nil {
			return nil, nil, err
		}
	}
	for _, extension := range pipeline.extensions {
		finalizer, ok := extension.Binder.(annotationBindingFinalizer)
		if !ok {
			continue
		}
		if err := finalizer.FinalizeAnnotationBinding(ctx); err != nil {
			return nil, nil, err
		}
	}
	return pkg, ctx.values, nil
}

func parseAnnotationPackages(fset *token.FileSet, dir string, files []string) (map[string]*ast.Package, error) {
	if len(files) == 0 {
		return parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
			name := info.Name()
			return !strings.HasSuffix(name, "_test.go") && strings.HasSuffix(name, ".go")
		}, parser.ParseComments)
	}
	packages := make(map[string]*ast.Package)
	for _, name := range files {
		name = strings.TrimSpace(name)
		if name == "" || strings.HasSuffix(name, "_test.go") || !strings.HasSuffix(name, ".go") {
			return nil, fmt.Errorf("invalid Go source file %q", name)
		}
		path := filepath.Join(dir, name)
		relative, err := filepath.Rel(dir, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("Go source file %q is outside scan directory", name)
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		packageName := file.Name.Name
		parsedPackage := packages[packageName]
		if parsedPackage == nil {
			parsedPackage = &ast.Package{Name: packageName, Files: make(map[string]*ast.File)}
			packages[packageName] = parsedPackage
		}
		parsedPackage.Files[path] = file
	}
	return packages, nil
}

func sortedPackageFiles(fset *token.FileSet, parsedPackage *ast.Package) []*ast.File {
	files := make([]*ast.File, 0, len(parsedPackage.Files))
	for _, file := range parsedPackage.Files {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		return fset.Position(files[i].Package).Filename < fset.Position(files[j].Package).Filename
	})
	return files
}

func scanAnnotationFile(ctx *AnnotationBindingContext, pipeline *annotationPipeline, fset *token.FileSet, file *ast.File) error {
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

func scanTypeDeclaration(ctx *AnnotationBindingContext, pipeline *annotationPipeline, fset *token.FileSet, file *ast.File, decl *ast.GenDecl) error {
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
			if err := scanInterfaceMethods(ctx, pipeline, fset, file, decl, typeSpec, interfaceType); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanStructFields(ctx *AnnotationBindingContext, pipeline *annotationPipeline, fset *token.FileSet, file *ast.File, decl *ast.GenDecl, typeSpec *ast.TypeSpec, structType *ast.StructType) error {
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

func scanInterfaceMethods(ctx *AnnotationBindingContext, pipeline *annotationPipeline, fset *token.FileSet, file *ast.File, decl *ast.GenDecl, typeSpec *ast.TypeSpec, interfaceType *ast.InterfaceType) error {
	for _, method := range interfaceType.Methods.List {
		methodAnnotations, err := parseAnnotations(method.Doc)
		if err != nil {
			return err
		}
		if len(methodAnnotations) == 0 {
			continue
		}
		item := AnnotationItem{
			target:      AnnotationTargetMethod,
			packageName: ctx.PackageName(),
			fset:        fset,
			file:        file,
			genDecl:     decl,
			typeSpec:    typeSpec,
			field:       method,
			annotations: methodAnnotations,
		}
		if err := pipeline.dispatch(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func mergeAnnotations(left []Annotation, right []Annotation) []Annotation {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	out := make([]Annotation, 0, len(left)+len(right))
	out = append(out, left...)
	out = append(out, right...)
	return out
}

func (p *annotationPipeline) dispatch(ctx *AnnotationBindingContext, item AnnotationItem) error {
	if err := p.validate(item); err != nil {
		return err
	}
	for _, extension := range p.extensions {
		if extension.Binder == nil || !itemMatchesDescriptors(item, extension.Descriptors) {
			continue
		}
		if err := extension.Binder.BindAnnotation(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (p *annotationPipeline) validate(item AnnotationItem) error {
	for _, annotation := range item.annotations {
		descriptor, ok := p.descriptors[annotation.Name]
		if !ok {
			return fmt.Errorf("unknown annotation %q", annotation.Name)
		}
		if !descriptorAllowsTarget(descriptor, item.Target()) {
			return fmt.Errorf("annotation %q does not support %s target", annotation.Name, item.Target())
		}
		if descriptor.Validate == nil {
			continue
		}
		ctx := AnnotationValidationContext{
			Target:     item.Target(),
			Annotation: annotation,
			Item:       item,
		}
		if err := descriptor.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}

func descriptorAllowsTarget(descriptor AnnotationDescriptor, target AnnotationTarget) bool {
	if len(descriptor.Targets) == 0 {
		return true
	}
	for _, item := range descriptor.Targets {
		if item == target {
			return true
		}
	}
	return false
}

func itemMatchesDescriptors(item AnnotationItem, descriptors []AnnotationDescriptor) bool {
	for _, descriptor := range descriptors {
		if item.HasAnnotation(descriptor.Name) {
			return true
		}
	}
	return false
}

func renderAnnotationPackage(pkg *annotationPackage, values map[string]any, pipeline *annotationPipeline) ([]byte, error) {
	ctx := &AnnotationGenerationContext{
		pkg:        pkg,
		values:     values,
		importKeys: make(map[string]struct{}),
	}
	for _, extension := range pipeline.extensions {
		if extension.Generator == nil {
			continue
		}
		if err := extension.Generator.GenerateAnnotation(ctx); err != nil {
			return nil, err
		}
	}

	var builder bytes.Buffer
	builder.WriteString("// Code generated by goark; DO NOT EDIT.\n")
	builder.WriteString("// 本文件由 goark 注解扫描生成，请勿手工修改。\n\n")
	builder.WriteString("package ")
	builder.WriteString(pkg.PackageName)
	builder.WriteString("\n\n")
	writeAnnotationImports(&builder, ctx.imports)
	builder.Write(ctx.body.Bytes())
	formatted, err := format.Source(builder.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated annotations: %w\n%s", err, builder.String())
	}
	return formatted, nil
}

func writeAnnotationImports(builder *bytes.Buffer, imports []ImportSpec) {
	if len(imports) == 0 {
		return
	}
	sortImports(imports)
	builder.WriteString("import (\n")
	for _, item := range imports {
		if item.Alias != "" {
			builder.WriteString(item.Alias)
			builder.WriteByte(' ')
		}
		builder.WriteString(strconv.Quote(item.Path))
		builder.WriteByte('\n')
	}
	builder.WriteString(")\n\n")
}

// DefaultAnnotationOutputName 返回注解扫描默认输出文件名。
func DefaultAnnotationOutputName(packageName string) string {
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		packageName = "package"
	}
	return filepath.Clean("zz_goark_" + packageName + "_gen.go")
}
