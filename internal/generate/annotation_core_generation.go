package generate

import (
	"fmt"
	"go/ast"
	"go/token"
)

func (coreAnnotationGenerator) GenerateAnnotation(ctx *AnnotationGenerationContext) error {
	value, ok := ctx.Value(coreAnnotationModelKey)
	if !ok {
		return nil
	}
	model, ok := value.(*coreAnnotationModel)
	if !ok {
		return fmt.Errorf("invalid core annotation model")
	}
	ctx.AddImport("", "context")
	ctx.AddImport("", "goark.dev/goark")
	ctx.AddImport("", "goark.dev/goark/container")
	if modelUsesOptionalInjection(model) {
		ctx.AddImport("arkerrors", "goark.dev/goark/errors")
	}
	if model.UsesProperties {
		ctx.AddImport("coreenv", "goark.dev/goark/core/env")
		ctx.AddImport("", "goark.dev/goark/core/resource")
	}
	if len(model.ConfigurationProperties) > 0 {
		ctx.AddImport("coreenv", "goark.dev/goark/core/env")
		ctx.AddImport("arkerrors", "goark.dev/goark/errors")
		addConfigurationPropertiesImports(ctx, model.ConfigurationProperties)
	}
	for _, properties := range model.ConfigurationProperties {
		writeConfigurationProperties(ctx.buffer(), properties)
	}
	for _, configuration := range model.Configurations {
		writeGeneratedConfiguration(ctx.buffer(), configuration)
	}
	return nil
}

func modelUsesOptionalInjection(model *coreAnnotationModel) bool {
	for _, configuration := range model.Configurations {
		for _, component := range configuration.Components {
			for _, field := range component.Fields {
				if !field.Injection.Required && field.Injection.Kind != "value" {
					return true
				}
			}
		}
		for _, bean := range configuration.Beans {
			for _, param := range bean.Params {
				if !param.Injection.Required && param.Injection.Kind != "value" {
					return true
				}
			}
		}
	}
	return false
}

func buildConfiguration(typeName string, annotations []Annotation) *annotationConfiguration {
	name := annotationName(annotations, "configuration", lowerCamel(typeName))
	return &annotationConfiguration{
		TypeName:        typeName,
		Name:            name,
		Order:           annotationInt(annotations, "order", 0),
		Profiles:        annotationStrings(annotations, "profile"),
		PropertySources: propertySourceAnnotations(annotations),
	}
}

func buildComponent(fset *token.FileSet, typeSpec *ast.TypeSpec, annotations []Annotation) (annotationComponent, bool, error) {
	typeName := typeSpec.Name.Name
	component := annotationComponent{
		TypeName:  typeName,
		Name:      annotationName(annotations, componentKind(annotations), lowerCamel(typeName)),
		Options:   buildBeanOptions(annotations),
		Profiles:  annotationStrings(annotations, "profile"),
		Condition: annotationString(annotations, "conditional", ""),
	}
	usesValue := false
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return component, false, nil
	}
	for _, field := range structType.Fields.List {
		fieldAnnotations, err := parseAnnotations(field.Doc)
		if err != nil {
			return annotationComponent{}, false, err
		}
		if len(field.Names) == 0 {
			continue
		}
		for _, name := range field.Names {
			injection := buildInjection(fieldAnnotations, name.Name)
			if injection.Kind == "" {
				continue
			}
			component.Fields = append(component.Fields, annotationField{
				Name:      name.Name,
				Type:      exprString(fset, field.Type),
				Injection: injection,
			})
			usesValue = usesValue || injection.Kind == "value"
		}
	}
	return component, usesValue, nil
}

func buildBean(fset *token.FileSet, fn *ast.FuncDecl, annotations []Annotation) (annotationBean, bool, error) {
	returnType, returnsError, err := beanReturnType(fset, fn.Type.Results)
	if err != nil {
		return annotationBean{}, false, err
	}
	bean := annotationBean{
		Name:         annotationName(annotations, "bean", lowerCamel(fn.Name.Name)),
		MethodName:   fn.Name.Name,
		ReturnType:   returnType,
		ReturnsError: returnsError,
		Options:      buildBeanOptions(annotations),
		Profiles:     annotationStrings(annotations, "profile"),
		Condition:    annotationString(annotations, "conditional", ""),
	}
	paramAnnotations := annotationsBySelector(annotations)
	usesValue := false
	if fn.Type.Params != nil {
		for index, field := range fn.Type.Params.List {
			names := field.Names
			if len(names) == 0 {
				names = []*ast.Ident{ast.NewIdent(fmt.Sprintf("arg%d", index))}
			}
			for _, name := range names {
				injection := buildInjection(paramAnnotations[name.Name], name.Name)
				if injection.Kind == "" {
					injection = injectionSpec{Kind: "bean", Required: true}
				}
				bean.Params = append(bean.Params, annotationParam{
					Name:      name.Name,
					Type:      exprString(fset, field.Type),
					Injection: injection,
				})
				usesValue = usesValue || injection.Kind == "value"
			}
		}
	}
	return bean, usesValue, nil
}

func beanReturnType(fset *token.FileSet, results *ast.FieldList) (string, bool, error) {
	if results == nil || len(results.List) == 0 {
		return "", false, fmt.Errorf("bean method must return T or (T, error)")
	}
	if len(results.List) == 1 {
		return exprString(fset, results.List[0].Type), false, nil
	}
	if len(results.List) == 2 && exprString(fset, results.List[1].Type) == "error" {
		return exprString(fset, results.List[0].Type), true, nil
	}
	return "", false, fmt.Errorf("bean method must return T or (T, error)")
}
