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
	"unicode"
	"unicode/utf8"
)

func parseAnnotationPackages(
	fset *token.FileSet,
	dir string,
	files []string,
) (map[string]*ast.Package, error) {
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
		if err != nil || relative == ".." ||
			strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
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

const sourcePackageAlias = "goarksource"

func prepareExternalGeneration(pkg *annotationPackage, values map[string]any) error {
	if pkg.SourceImportPath == "" {
		return nil
	}
	if err := validateExternalSymbols(pkg, values); err != nil {
		return err
	}
	qualifyCoreModel(pkg, values)
	qualifyWebModel(pkg, values)
	qualifyMVCModel(pkg, values)
	return nil
}

func validateExternalSymbols(pkg *annotationPackage, values map[string]any) error {
	for name := range pkg.types {
		if !ast.IsExported(name) && annotatedTypeName(values, name) {
			return fmt.Errorf("生成 gen 包要求注解类型 %s 可导出", name)
		}
	}
	model, _ := values[coreAnnotationModelKey].(*coreAnnotationModel)
	if model == nil {
		return nil
	}
	for _, configuration := range model.Configurations {
		for _, component := range configuration.Components {
			for _, field := range component.Fields {
				if !ast.IsExported(field.Name) {
					return fmt.Errorf("生成 gen 包要求注入字段 %s.%s 可导出", component.TypeName, field.Name)
				}
			}
		}
		for _, bean := range configuration.Beans {
			if !ast.IsExported(bean.MethodName) {
				return fmt.Errorf(
					"生成 gen 包要求 Bean 方法 %s.%s 可导出",
					configuration.TypeName,
					bean.MethodName,
				)
			}
		}
	}
	web, _ := values[webAnnotationModelKey].(*webAnnotationModel)
	if web != nil {
		for _, item := range webModelComponents(web) {
			if err := validateExternalComponent(item.Component); err != nil {
				return err
			}
		}
	}
	mvcModel, _ := values[mvcAnnotationModelKey].(*mvcAnnotationModel)
	if mvcModel != nil {
		for _, controller := range mvcModel.Controllers {
			if err := validateExternalComponent(controller.Component); err != nil {
				return err
			}
			for _, route := range controller.Routes {
				if !ast.IsExported(route.MethodName) {
					return fmt.Errorf("生成 gen 包要求 MVC 方法 %s 可导出", route.MethodName)
				}
			}
			for _, attribute := range controller.ModelAttributes {
				if !ast.IsExported(attribute.MethodName) {
					return fmt.Errorf("生成 gen 包要求 MVC 方法 %s 可导出", attribute.MethodName)
				}
			}
		}
		for _, advice := range mvcModel.Advices {
			if err := validateExternalComponent(advice.Component); err != nil {
				return err
			}
			for _, handler := range advice.ExceptionHandlers {
				if !ast.IsExported(handler.MethodName) {
					return fmt.Errorf("生成 gen 包要求异常处理方法 %s 可导出", handler.MethodName)
				}
			}
		}
	}
	return nil
}

func validateExternalComponent(component annotationComponent) error {
	for _, field := range component.Fields {
		if !ast.IsExported(field.Name) {
			return fmt.Errorf("生成 gen 包要求注入字段 %s.%s 可导出", component.TypeName, field.Name)
		}
	}
	return nil
}

func annotatedTypeName(values map[string]any, name string) bool {
	core, _ := values[coreAnnotationModelKey].(*coreAnnotationModel)
	if core != nil {
		for _, item := range core.Configurations {
			if item.TypeName == name {
				return true
			}
			for _, component := range item.Components {
				if component.TypeName == name {
					return true
				}
			}
		}
		for _, item := range core.ConfigurationProperties {
			if item.TypeName == name {
				return true
			}
		}
	}
	web, _ := values[webAnnotationModelKey].(*webAnnotationModel)
	if web != nil {
		for _, item := range webModelComponents(web) {
			if item.Component.TypeName == name {
				return true
			}
		}
	}
	mvcModel, _ := values[mvcAnnotationModelKey].(*mvcAnnotationModel)
	if mvcModel != nil {
		for _, item := range mvcModel.Controllers {
			if item.Component.TypeName == name {
				return true
			}
		}
		for _, item := range mvcModel.Advices {
			if item.Component.TypeName == name {
				return true
			}
		}
	}
	return false
}

func qualifyCoreModel(pkg *annotationPackage, values map[string]any) {
	model, _ := values[coreAnnotationModelKey].(*coreAnnotationModel)
	if model == nil {
		return
	}
	for index := range model.ConfigurationProperties {
		properties := &model.ConfigurationProperties[index]
		qualifyProperties(pkg, properties)
	}
	for _, configuration := range model.Configurations {
		if !configuration.Synthetic {
			configuration.SourceTypeName = qualifyLocalIdentifiers(
				configuration.TypeName,
				pkg.types,
			)
			configuration.Synthetic = true
		}
		qualifyConfiguration(pkg, configuration)
	}
}

func qualifyConfiguration(pkg *annotationPackage, configuration *annotationConfiguration) {
	for index := range configuration.Properties {
		qualifyProperties(pkg, &configuration.Properties[index])
	}
	for index := range configuration.Components {
		qualifyComponent(pkg, &configuration.Components[index])
	}
	for index := range configuration.Beans {
		bean := &configuration.Beans[index]
		bean.ReturnType = qualifyLocalIdentifiers(bean.ReturnType, pkg.types)
		bean.Condition = qualifyLocalIdentifiers(bean.Condition, pkg.types)
		bean.ConfigurationType = configuration.SourceTypeName
		for item := range bean.Params {
			bean.Params[item].Type = qualifyLocalIdentifiers(bean.Params[item].Type, pkg.types)
		}
	}
}

func qualifyProperties(pkg *annotationPackage, properties *annotationConfigurationProperties) {
	properties.SourceTypeName = qualifyLocalIdentifiers(properties.TypeName, pkg.types)
	for item := range properties.Initializers {
		properties.Initializers[item] = qualifyLocalIdentifiers(
			properties.Initializers[item],
			pkg.types,
		)
	}
	for item := range properties.Fields {
		properties.Fields[item].Type = qualifyLocalIdentifiers(
			properties.Fields[item].Type,
			pkg.types,
		)
		properties.Fields[item].MapValueType = qualifyLocalIdentifiers(
			properties.Fields[item].MapValueType,
			pkg.types,
		)
	}
}

func qualifyComponent(pkg *annotationPackage, component *annotationComponent) {
	component.TypeName = qualifyLocalIdentifiers(component.TypeName, pkg.types)
	component.Condition = qualifyLocalIdentifiers(component.Condition, pkg.types)
	for index := range component.Fields {
		component.Fields[index].Type = qualifyLocalIdentifiers(
			component.Fields[index].Type,
			pkg.types,
		)
	}
}

func qualifyWebModel(pkg *annotationPackage, values map[string]any) {
	model, _ := values[webAnnotationModelKey].(*webAnnotationModel)
	if model == nil {
		return
	}
	for _, item := range webModelComponents(model) {
		qualifyComponent(pkg, &item.Component)
	}
}

func qualifyMVCModel(pkg *annotationPackage, values map[string]any) {
	model, _ := values[mvcAnnotationModelKey].(*mvcAnnotationModel)
	if model == nil {
		return
	}
	for _, controller := range model.Controllers {
		qualifyComponent(pkg, &controller.Component)
		for index := range controller.Routes {
			qualifyMVCRoute(pkg, &controller.Routes[index])
		}
		for index := range controller.ModelAttributes {
			attribute := &controller.ModelAttributes[index]
			attribute.ReturnType = qualifyLocalIdentifiers(attribute.ReturnType, pkg.types)
			qualifyMVCParams(pkg, attribute.Params)
		}
	}
	for _, advice := range model.Advices {
		qualifyComponent(pkg, &advice.Component)
		for index := range advice.ExceptionHandlers {
			handler := &advice.ExceptionHandlers[index]
			handler.ErrorType = qualifyLocalIdentifiers(handler.ErrorType, pkg.types)
			handler.EntityBody = qualifyLocalIdentifiers(handler.EntityBody, pkg.types)
		}
	}
}

func qualifyMVCRoute(pkg *annotationPackage, route *mvcRoute) {
	route.Handler.ReturnType = qualifyLocalIdentifiers(route.Handler.ReturnType, pkg.types)
	route.Handler.EntityBody = qualifyLocalIdentifiers(route.Handler.EntityBody, pkg.types)
	qualifyMVCParams(pkg, route.Handler.Params)
}

func qualifyMVCParams(pkg *annotationPackage, params []mvcHandlerParam) {
	for index := range params {
		params[index].Type = qualifyLocalIdentifiers(params[index].Type, pkg.types)
		params[index].BodyType = qualifyLocalIdentifiers(params[index].BodyType, pkg.types)
	}
}

func qualifyLocalIdentifiers(value string, types map[string]annotationTypeDeclaration) string {
	if value == "" {
		return value
	}
	var builder strings.Builder
	builder.Grow(len(value) + len(sourcePackageAlias) + 1)
	for index := 0; index < len(value); {
		current, size := utf8.DecodeRuneInString(value[index:])
		if current != '_' && !unicode.IsLetter(current) {
			builder.WriteString(value[index : index+size])
			index += size
			continue
		}
		start := index
		for index < len(value) {
			current, size = utf8.DecodeRuneInString(value[index:])
			if current != '_' && !unicode.IsLetter(current) && !unicode.IsDigit(current) {
				break
			}
			index += size
		}
		name := value[start:index]
		previousIsSelector := start > 0 && value[start-1] == '.'
		if _, ok := types[name]; ok && !previousIsSelector {
			builder.WriteString(sourcePackageAlias)
			builder.WriteByte('.')
		}
		builder.WriteString(name)
	}
	return builder.String()
}
