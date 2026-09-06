package generate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
