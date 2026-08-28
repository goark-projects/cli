package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strconv"
)

const webAnnotationModelKey = "goark.web.annotations"

const (
	webInterceptorAnnotation = "web-interceptor"
	webFilterAnnotation      = "web-filter"
)

type webAnnotationModel struct {
	Interceptors []*webComponent
	Filters      []*webComponent
	byType       map[string]*webComponent
}

type webComponent struct {
	Component annotationComponent
	Kind      string
}

type webAnnotationBinder struct{}

type webAnnotationGenerator struct{}

func webAnnotationExtension() AnnotationExtension {
	return AnnotationExtension{
		Descriptors: webAnnotationDescriptors(),
		Binder:      webAnnotationBinder{},
		Generator:   webAnnotationGenerator{},
	}
}

func webAnnotationDescriptors() []AnnotationDescriptor {
	return []AnnotationDescriptor{
		{Name: webInterceptorAnnotation, Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateWebComponentAnnotation},
		{Name: webFilterAnnotation, Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateWebComponentAnnotation},
	}
}

func validateWebComponentAnnotation(ctx AnnotationValidationContext) error {
	if err := validateCoreStructTypeAnnotation(ctx); err != nil {
		return err
	}
	if countWebComponentAnnotations(ctx.Item.Annotations()) > 1 {
		return fmt.Errorf("web component type %q must declare exactly one web interceptor or filter annotation", ctx.Item.TypeName())
	}
	return validateCoreNameAnnotation(ctx.Annotation)
}

func (webAnnotationBinder) BindAnnotation(ctx *AnnotationBindingContext, item AnnotationItem) error {
	if item.Target() != AnnotationTargetType {
		return nil
	}
	kind := webComponentKind(item.Annotations())
	if kind == "" {
		return nil
	}
	component, err := buildWebComponent(item.FileSet(), item.TypeSpec(), item.Annotations(), kind)
	if err != nil {
		return err
	}
	model := ensureWebAnnotationModel(ctx)
	if _, exists := model.byType[component.Component.TypeName]; exists {
		return fmt.Errorf("duplicate web component type %q", component.Component.TypeName)
	}
	model.byType[component.Component.TypeName] = component
	switch kind {
	case webInterceptorAnnotation:
		model.Interceptors = append(model.Interceptors, component)
	case webFilterAnnotation:
		model.Filters = append(model.Filters, component)
	}
	return nil
}

func (webAnnotationBinder) FinalizeAnnotationBinding(ctx *AnnotationBindingContext) error {
	value, ok := ctx.Value(webAnnotationModelKey)
	if !ok {
		return nil
	}
	model, ok := value.(*webAnnotationModel)
	if !ok {
		return fmt.Errorf("invalid web annotation model")
	}
	coreModel := ensureCoreAnnotationModel(ctx)
	resolver := newAnnotationDependencyResolver(coreModel)
	for _, component := range webModelComponents(model) {
		resolver.addCandidate(annotationDependencyCandidate{
			Name:     component.Component.Name,
			Type:     "*" + component.Component.TypeName,
			Primary:  component.Component.Options.Primary,
			Priority: component.Component.Options.Priority,
		})
	}
	seenNames := make(map[string]string, len(model.Interceptors)+len(model.Filters))
	for _, component := range webModelComponents(model) {
		if existing := seenNames[component.Component.Name]; existing != "" {
			return fmt.Errorf("duplicate web component name %q for %s and %s", component.Component.Name, existing, component.Component.TypeName)
		}
		seenNames[component.Component.Name] = component.Component.TypeName
		inferComponentDependencyMetadata(&component.Component, resolver)
	}
	sortWebComponents(model.Interceptors)
	sortWebComponents(model.Filters)
	return nil
}

func ensureWebAnnotationModel(ctx *AnnotationBindingContext) *webAnnotationModel {
	if value, ok := ctx.Value(webAnnotationModelKey); ok {
		if model, ok := value.(*webAnnotationModel); ok {
			return model
		}
	}
	model := &webAnnotationModel{byType: make(map[string]*webComponent)}
	ctx.SetValue(webAnnotationModelKey, model)
	return model
}

func (webAnnotationGenerator) GenerateAnnotation(ctx *AnnotationGenerationContext) error {
	value, ok := ctx.Value(webAnnotationModelKey)
	if !ok {
		return nil
	}
	model, ok := value.(*webAnnotationModel)
	if !ok {
		return fmt.Errorf("invalid web annotation model")
	}
	if len(model.Interceptors) == 0 && len(model.Filters) == 0 {
		return nil
	}
	ctx.AddImport("", "context")
	ctx.AddImport("", "goark.dev/goark")
	ctx.AddImport("", "goark.dev/goark/container")
	ctx.AddImport("goweb", "goark.dev/goark/web")
	if webModelUsesOptionalInjection(model) {
		ctx.AddImport("arkerrors", "goark.dev/goark/errors")
	}
	writeWebConfiguration(ctx.buffer(), model)
	return nil
}

func buildWebComponent(fset *token.FileSet, typeSpec *ast.TypeSpec, annotations []Annotation, kind string) (*webComponent, error) {
	typeName := typeSpec.Name.Name
	component := annotationComponent{
		TypeName:  typeName,
		Name:      annotationName(annotations, kind, lowerCamel(typeName)),
		Options:   buildBeanOptions(annotations),
		Profiles:  annotationStrings(annotations, "profile"),
		Condition: annotationString(annotations, "conditional", ""),
	}
	structType, _ := typeSpec.Type.(*ast.StructType)
	if structType == nil {
		return &webComponent{Component: component, Kind: kind}, nil
	}
	for _, field := range structType.Fields.List {
		fieldAnnotations, err := parseAnnotations(field.Doc)
		if err != nil {
			return nil, err
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
		}
	}
	return &webComponent{Component: component, Kind: kind}, nil
}

func writeWebConfiguration(builder *bytes.Buffer, model *webAnnotationModel) {
	builder.WriteString("type GoarkWebConfiguration struct{}\n\n")
	builder.WriteString("func (GoarkWebConfiguration) Name() string {\nreturn \"goark.web\"\n}\n\n")
	builder.WriteString("func (GoarkWebConfiguration) Order() int {\nreturn 0\n}\n\n")
	builder.WriteString("func (c GoarkWebConfiguration) Register(ctx context.Context, registry *container.Registry) error {\n")
	builder.WriteString("return c.RegisterWithContext(ctx, goark.NewConfigurationContext(nil, registry))\n")
	builder.WriteString("}\n\n")
	builder.WriteString("func (c GoarkWebConfiguration) RegisterWithContext(ctx context.Context, config goark.ConfigurationContext) error {\n")
	builder.WriteString("registry := config.Registry()\n")
	for _, component := range webModelComponents(model) {
		writeComponentRegistration(builder, component.Component)
	}
	for _, component := range model.Interceptors {
		writeWebInterceptorConfigurerRegistration(builder, component.Component)
	}
	for _, component := range model.Filters {
		writeWebFilterConfigurerRegistration(builder, component.Component)
	}
	builder.WriteString("return nil\n}\n\n")
}

func writeWebInterceptorConfigurerRegistration(builder *bytes.Buffer, component annotationComponent) {
	writeWebConfigurerRegistration(builder, component, "webInterceptorConfigurer", "interceptor", "Use")
}

func writeWebFilterConfigurerRegistration(builder *bytes.Buffer, component annotationComponent) {
	writeWebConfigurerRegistration(builder, component, "webFilterConfigurer", "filter", "AddFilter")
}

func writeWebConfigurerRegistration(builder *bytes.Buffer, component annotationComponent, suffix string, variableName string, registryMethod string) {
	writeConditionalStart(builder, component.Name, component.Profiles, component.Condition)
	configurerName := component.Name + "." + suffix
	builder.WriteString("if err := container.Register[goweb.Configurer](registry, ")
	builder.WriteString(strconv.Quote(configurerName))
	builder.WriteString(", func(ctx context.Context, resolver container.Resolver) (out goweb.Configurer, err error) {\n")
	builder.WriteString(variableName)
	builder.WriteString(", err := container.GetByType[*")
	builder.WriteString(component.TypeName)
	builder.WriteString("](ctx, resolver, container.WithQualifier(")
	builder.WriteString(strconv.Quote(component.Name))
	builder.WriteString("))\nif err != nil {\nreturn nil, err\n}\n")
	builder.WriteString("out = goweb.ConfigurerFunc(func(ctx context.Context, webRegistry *goweb.Registry) error {\n")
	builder.WriteString("if err := ctx.Err(); err != nil {\nreturn err\n}\n")
	builder.WriteString("if webRegistry == nil {\nreturn goweb.ErrNilRegistry\n}\n")
	builder.WriteString("webRegistry.")
	builder.WriteString(registryMethod)
	builder.WriteByte('(')
	builder.WriteString(variableName)
	builder.WriteString(")\nreturn nil\n})\nreturn out, nil\n}")
	writeContainerOptions(builder, webConfigurerOptions(component))
	builder.WriteString("); err != nil {\nreturn err\n}\n")
	writeConditionalEnd(builder, component.Profiles, component.Condition)
}

func webConfigurerOptions(component annotationComponent) annotationBeanOptions {
	return annotationBeanOptions{
		FactoryDependencies: []string{component.Name},
		Order:               component.Options.Order,
		Priority:            component.Options.Priority,
	}
}

func webComponentKind(annotations []Annotation) string {
	for _, name := range []string{webInterceptorAnnotation, webFilterAnnotation} {
		if hasAnnotation(annotations, name) {
			return name
		}
	}
	return ""
}

func countWebComponentAnnotations(annotations []Annotation) int {
	count := 0
	for _, annotation := range annotations {
		switch annotation.Name {
		case webInterceptorAnnotation, webFilterAnnotation:
			count++
		}
	}
	return count
}

func webModelComponents(model *webAnnotationModel) []*webComponent {
	components := make([]*webComponent, 0, len(model.Interceptors)+len(model.Filters))
	components = append(components, model.Interceptors...)
	components = append(components, model.Filters...)
	sortWebComponents(components)
	return components
}

func sortWebComponents(components []*webComponent) {
	sort.SliceStable(components, func(i, j int) bool {
		return components[i].Component.Name < components[j].Component.Name
	})
}

func webModelUsesOptionalInjection(model *webAnnotationModel) bool {
	for _, component := range webModelComponents(model) {
		for _, field := range component.Component.Fields {
			if !field.Injection.Required && field.Injection.Kind != "value" {
				return true
			}
		}
	}
	return false
}
