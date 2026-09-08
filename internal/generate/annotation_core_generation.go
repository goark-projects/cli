package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

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

func buildComponent(
	fset *token.FileSet,
	typeSpec *ast.TypeSpec,
	annotations []Annotation,
) (annotationComponent, bool, error) {
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
		fieldAnnotations, err := parseAnnotations(field.Doc, field.Comment)
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

func buildBean(
	fset *token.FileSet,
	fn *ast.FuncDecl,
	annotations []Annotation,
) (annotationBean, bool, error) {
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

func componentKind(annotations []Annotation) string {
	for _, name := range []string{"component", "service", "repository"} {
		if hasAnnotation(annotations, name) {
			return name
		}
	}
	return ""
}

func componentOptionKind(annotations []Annotation) string {
	if kind := componentKind(annotations); kind != "" {
		return kind
	}
	return webComponentKind(annotations)
}

func buildBeanOptions(annotations []Annotation) annotationBeanOptions {
	options := annotationBeanOptions{
		Primary: hasAnnotation(annotations, "primary"),
		Lazy:    annotationBoolByName(annotations, "lazy", true),
		Scope:   annotationString(annotations, "scope", ""),
	}
	if !hasAnnotation(annotations, "lazy") {
		options.Lazy = false
	}
	if values := annotationStrings(annotations, "depends-on"); len(values) > 0 {
		for _, value := range values {
			for _, item := range strings.Split(value, ",") {
				item = strings.TrimSpace(item)
				if item != "" {
					options.DependsOn = append(options.DependsOn, item)
				}
			}
		}
	}
	if hasAnnotation(annotations, "order") {
		value := annotationInt(annotations, "order", 0)
		options.Order = &value
	}
	if hasAnnotation(annotations, "priority") {
		value := annotationInt(annotations, "priority", 0)
		options.Priority = &value
	}
	return options
}

func propertySourceAnnotations(annotations []Annotation) []annotationPropertySource {
	sources := make([]annotationPropertySource, 0)
	for _, annotation := range annotations {
		switch annotation.Name {
		case "property-source":
			source := annotationPropertySource{
				Location:               argString(annotation, "value", ""),
				Name:                   argString(annotation, "name", ""),
				Encoding:               argString(annotation, "encoding", ""),
				IgnoreResourceNotFound: annotationBool(annotation, "ignoreResourceNotFound", false),
			}
			if source.Location != "" {
				sources = append(sources, source)
			}
		case "property-sources":
			for _, location := range strings.Split(argString(annotation, "value", ""), ";") {
				location = strings.TrimSpace(location)
				if location != "" {
					sources = append(sources, annotationPropertySource{Location: location})
				}
			}
		}
	}
	return sources
}

func buildInjection(annotations []Annotation, defaultName string) injectionSpec {
	injection := injectionSpec{Required: true}
	if value := annotationString(annotations, "value", ""); value != "" {
		injection.Kind = "value"
		injection.Value = value
		return injection
	}
	qualifier := firstNonEmpty(
		annotationString(annotations, "qualifier", ""),
		annotationString(annotations, "named", ""),
		autowiredQualifier(annotations),
	)
	if hasAnnotation(annotations, "resource") {
		injection.Kind = "resource"
		injection.Qualifier = annotationString(annotations, "resource", defaultName)
		return injection
	}
	if hasAnnotation(annotations, "inject") || hasAnnotation(annotations, "autowired") ||
		qualifier != "" {
		injection.Kind = "bean"
		injection.Qualifier = qualifier
		if hasAnnotation(annotations, "autowired") {
			injection.Required = autowiredRequired(annotations)
		}
	}
	return injection
}

func autowiredQualifier(annotations []Annotation) string {
	for _, annotation := range annotations {
		if annotation.Name != "autowired" {
			continue
		}
		if value, ok := annotation.Args["qualifier"]; ok {
			return value.Text()
		}
	}
	return ""
}

func autowiredRequired(annotations []Annotation) bool {
	for _, annotation := range annotations {
		if annotation.Name != "autowired" {
			continue
		}
		return annotationBool(annotation, "required", true)
	}
	return true
}

func writeGeneratedConfiguration(builder *bytes.Buffer, configuration *annotationConfiguration) {
	if configuration.SourceTypeName != "" {
		builder.WriteString("type ")
		builder.WriteString(configuration.TypeName)
		builder.WriteString(" struct {\nsource ")
		builder.WriteString(configuration.SourceTypeName)
		builder.WriteString("\n}\n\n")
	} else if configuration.Synthetic {
		builder.WriteString("type ")
		builder.WriteString(configuration.TypeName)
		builder.WriteString(" struct{}\n\n")
	}
	builder.WriteString("func (")
	builder.WriteString(configuration.TypeName)
	builder.WriteString(") Name() string {\nreturn ")
	builder.WriteString(strconv.Quote(configuration.Name))
	builder.WriteString("\n}\n\n")
	builder.WriteString("func (")
	builder.WriteString(configuration.TypeName)
	builder.WriteString(") Order() int {\nreturn ")
	builder.WriteString(strconv.Itoa(configuration.Order))
	builder.WriteString("\n}\n\n")
	if len(configuration.PropertySources) > 0 {
		writeConfigureEnvironment(builder, configuration)
	}
	writeRegisterWithContext(builder, configuration)
}

func writeConfigureEnvironment(builder *bytes.Buffer, configuration *annotationConfiguration) {
	builder.WriteString("func (")
	builder.WriteString(configuration.TypeName)
	builder.WriteString(
		") ConfigureEnvironment(ctx context.Context, " +
			"environment coreenv.ConfigurableEnvironment) error {\n",
	)
	builder.WriteString("loader, err := resource.NewLoader()\nif err != nil {\nreturn err\n}\n")
	for _, source := range configuration.PropertySources {
		builder.WriteString("source, err := coreenv.LoadPropertiesPropertySource(ctx, loader, ")
		builder.WriteString(strconv.Quote(source.Location))
		if source.Name != "" {
			builder.WriteString(", coreenv.WithPropertySourceName(")
			builder.WriteString(strconv.Quote(source.Name))
			builder.WriteString(")")
		}
		if source.Encoding != "" {
			builder.WriteString(", coreenv.WithPropertySourceEncoding(")
			builder.WriteString(strconv.Quote(source.Encoding))
			builder.WriteString(")")
		}
		if source.IgnoreResourceNotFound {
			builder.WriteString(", coreenv.WithIgnoreResourceNotFound(true)")
		}
		builder.WriteString(
			")\nif err != nil {\nreturn err\n}\nif source != nil {\n" +
				"if err := environment.PropertySources().AddLast(source); err != nil {\n" +
				"return err\n}\n}\n",
		)
	}
	builder.WriteString("return nil\n}\n\n")
}

func writeRegisterWithContext(builder *bytes.Buffer, configuration *annotationConfiguration) {
	builder.WriteString("func (c ")
	builder.WriteString(configuration.TypeName)
	builder.WriteString(") Register(ctx context.Context, registry *container.Registry) error {\n")
	builder.WriteString(
		"return c.RegisterWithContext(ctx, goark.NewConfigurationContext(nil, registry))\n",
	)
	builder.WriteString("}\n\n")
	builder.WriteString("func (c ")
	builder.WriteString(configuration.TypeName)
	builder.WriteString(
		") RegisterWithContext(ctx context.Context, config goark.ConfigurationContext) error {\n",
	)
	if len(configuration.Profiles) > 0 {
		writeProfileGuard(
			builder,
			strings.Join(wrapExpressions(configuration.Profiles), " | "),
			configuration.Name,
			"return nil",
		)
	}
	if len(configuration.Components) > 0 || len(configuration.Beans) > 0 ||
		len(configuration.Properties) > 0 {
		builder.WriteString("registry := config.Registry()\n")
	}
	for _, properties := range configuration.Properties {
		writeConfigurationPropertiesRegistration(builder, properties)
	}
	for _, component := range configuration.Components {
		writeComponentRegistration(builder, component)
	}
	for _, bean := range configuration.Beans {
		writeBeanRegistrationFromAnnotation(builder, bean)
	}
	builder.WriteString("return nil\n}\n\n")
}
