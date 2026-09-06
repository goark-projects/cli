package generate

import (
	"fmt"
	"go/ast"
	"sort"
	"strings"
)

func (coreAnnotationBinder) BindAnnotation(ctx *AnnotationBindingContext, item AnnotationItem) error {
	switch item.Target() {
	case AnnotationTargetType:
		return bindCoreTypeAnnotation(ctx, item)
	case AnnotationTargetMethod:
		return bindCoreMethodAnnotation(ctx, item)
	default:
		return nil
	}
}

func (coreAnnotationBinder) FinalizeAnnotationBinding(ctx *AnnotationBindingContext) error {
	model := ensureCoreAnnotationModel(ctx)
	if len(model.Configurations) == 0 {
		typeName := strings.TrimSpace(ctx.spec.TypeName)
		if typeName == "" {
			typeName = "GoarkPackageConfiguration"
		}
		name := strings.TrimSpace(ctx.spec.ConfigurationName)
		if name == "" {
			name = "goark.package." + ctx.PackageName()
		}
		configuration := &annotationConfiguration{
			TypeName:  typeName,
			Name:      name,
			Synthetic: true,
		}
		model.Configurations = append(model.Configurations, configuration)
	}
	sort.SliceStable(model.Configurations, func(i, j int) bool {
		return model.Configurations[i].TypeName < model.Configurations[j].TypeName
	})
	model.Configurations[0].Components = append(model.Configurations[0].Components, model.Components...)
	model.Configurations[0].Properties = append(model.Configurations[0].Properties, model.ConfigurationProperties...)
	for _, configuration := range model.Configurations {
		sort.SliceStable(configuration.Beans, func(i, j int) bool {
			return configuration.Beans[i].Name < configuration.Beans[j].Name
		})
		sort.SliceStable(configuration.Components, func(i, j int) bool {
			return configuration.Components[i].Name < configuration.Components[j].Name
		})
		if len(configuration.PropertySources) > 0 {
			model.UsesProperties = true
		}
	}
	inferCoreDependencyMetadata(model)
	return nil
}

func inferCoreDependencyMetadata(model *coreAnnotationModel) {
	resolver := newAnnotationDependencyResolver(model)
	for _, configuration := range model.Configurations {
		for index := range configuration.Components {
			inferComponentDependencyMetadata(&configuration.Components[index], resolver)
		}
		for index := range configuration.Beans {
			inferBeanDependencyMetadata(&configuration.Beans[index], resolver)
		}
	}
}

func newAnnotationDependencyResolver(model *coreAnnotationModel) annotationDependencyResolver {
	resolver := annotationDependencyResolver{
		byType: make(map[string][]annotationDependencyCandidate),
	}
	for _, configuration := range model.Configurations {
		for _, properties := range configuration.Properties {
			resolver.addCandidate(annotationDependencyCandidate{
				Name: properties.BeanName,
				Type: "*" + properties.TypeName,
			})
		}
		for _, component := range configuration.Components {
			resolver.addCandidate(annotationDependencyCandidate{
				Name:     component.Name,
				Type:     "*" + component.TypeName,
				Primary:  component.Options.Primary,
				Priority: component.Options.Priority,
			})
		}
		for _, bean := range configuration.Beans {
			resolver.addCandidate(annotationDependencyCandidate{
				Name:     bean.Name,
				Type:     bean.ReturnType,
				Primary:  bean.Options.Primary,
				Priority: bean.Options.Priority,
			})
		}
	}
	return resolver
}

func (r annotationDependencyResolver) addCandidate(candidate annotationDependencyCandidate) {
	candidate.Name = strings.TrimSpace(candidate.Name)
	candidate.Type = strings.TrimSpace(candidate.Type)
	if candidate.Name == "" || candidate.Type == "" {
		return
	}
	r.byType[candidate.Type] = append(r.byType[candidate.Type], candidate)
}

func inferComponentDependencyMetadata(component *annotationComponent, resolver annotationDependencyResolver) {
	for _, field := range component.Fields {
		name := resolver.dependencyName(field.Type, field.Injection)
		if name == "" {
			continue
		}
		if field.Injection.Required {
			component.Options.InjectionDependencies = appendUniqueDependency(component.Options.InjectionDependencies, name)
			continue
		}
		component.Options.OptionalInjectionDependencies = appendUniqueDependency(component.Options.OptionalInjectionDependencies, name)
	}
}

func inferBeanDependencyMetadata(bean *annotationBean, resolver annotationDependencyResolver) {
	for _, param := range bean.Params {
		name := resolver.dependencyName(param.Type, param.Injection)
		if name == "" {
			continue
		}
		bean.Options.FactoryDependencies = appendUniqueDependency(bean.Options.FactoryDependencies, name)
	}
}

func (r annotationDependencyResolver) dependencyName(typ string, injection injectionSpec) string {
	if injection.Kind != "bean" {
		return ""
	}
	if injection.Qualifier != "" {
		return injection.Qualifier
	}
	candidate, ok := r.resolveByType(typ)
	if !ok {
		return ""
	}
	return candidate.Name
}

func (r annotationDependencyResolver) resolveByType(typ string) (annotationDependencyCandidate, bool) {
	candidates := append([]annotationDependencyCandidate(nil), r.byType[strings.TrimSpace(typ)]...)
	switch len(candidates) {
	case 0:
		return annotationDependencyCandidate{}, false
	case 1:
		return candidates[0], true
	}
	if candidate, ok := uniquePrimaryCandidate(candidates); ok {
		return candidate, true
	}
	return uniqueHighestPriorityCandidate(candidates)
}

func uniquePrimaryCandidate(candidates []annotationDependencyCandidate) (annotationDependencyCandidate, bool) {
	var selected annotationDependencyCandidate
	count := 0
	for _, candidate := range candidates {
		if !candidate.Primary {
			continue
		}
		selected = candidate
		count++
	}
	return selected, count == 1
}

func uniqueHighestPriorityCandidate(candidates []annotationDependencyCandidate) (annotationDependencyCandidate, bool) {
	var selected annotationDependencyCandidate
	selectedSet := false
	ambiguous := false
	for _, candidate := range candidates {
		if candidate.Priority == nil {
			continue
		}
		if !selectedSet || *candidate.Priority < *selected.Priority {
			selected = candidate
			selectedSet = true
			ambiguous = false
			continue
		}
		if *candidate.Priority == *selected.Priority {
			ambiguous = true
		}
	}
	return selected, selectedSet && !ambiguous
}

func appendUniqueDependency(names []string, name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return names
	}
	for _, existing := range names {
		if existing == name {
			return names
		}
	}
	return append(names, name)
}

func bindCoreTypeAnnotation(ctx *AnnotationBindingContext, item AnnotationItem) error {
	typeSpec := item.TypeSpec()
	if typeSpec == nil {
		return nil
	}
	if _, ok := typeSpec.Type.(*ast.StructType); !ok {
		return nil
	}
	annotations := item.annotations
	model := ensureCoreAnnotationModel(ctx)
	if hasAnnotation(annotations, "configuration-properties") {
		properties, err := buildConfigurationProperties(ctx, item)
		if err != nil {
			return err
		}
		model.ConfigurationProperties = append(model.ConfigurationProperties, properties)
	}
	if hasAnnotation(annotations, "configuration") {
		configuration := buildConfiguration(typeSpec.Name.Name, annotations)
		model.Configurations = append(model.Configurations, configuration)
		model.configByType[configuration.TypeName] = configuration
		return nil
	}
	if hasAnnotation(annotations, "configuration-properties") {
		return nil
	}
	if componentKind(annotations) == "" {
		return nil
	}
	component, usesValue, err := buildComponent(item.FileSet(), typeSpec, annotations)
	if err != nil {
		return err
	}
	model.Components = append(model.Components, component)
	model.UsesValue = model.UsesValue || usesValue
	return nil
}

func bindCoreMethodAnnotation(ctx *AnnotationBindingContext, item AnnotationItem) error {
	if !item.HasAnnotation("bean") || item.FuncDecl() == nil || item.FuncDecl().Recv == nil {
		return nil
	}
	receiver := item.ReceiverTypeName()
	if receiver == "" {
		return fmt.Errorf("bean method %s receiver is not supported", item.FuncName())
	}
	model := ensureCoreAnnotationModel(ctx)
	configuration := model.configByType[receiver]
	if configuration == nil {
		configuration = &annotationConfiguration{
			TypeName: receiver,
			Name:     lowerCamel(strings.TrimSuffix(receiver, "Configuration")),
		}
		model.Configurations = append(model.Configurations, configuration)
		model.configByType[receiver] = configuration
	}
	bean, usesValue, err := buildBean(item.FileSet(), item.FuncDecl(), item.annotations)
	if err != nil {
		return err
	}
	configuration.Beans = append(configuration.Beans, bean)
	model.UsesValue = model.UsesValue || usesValue
	return nil
}

func ensureCoreAnnotationModel(ctx *AnnotationBindingContext) *coreAnnotationModel {
	if value, ok := ctx.Value(coreAnnotationModelKey); ok {
		if model, ok := value.(*coreAnnotationModel); ok {
			return model
		}
	}
	model := &coreAnnotationModel{configByType: make(map[string]*annotationConfiguration)}
	ctx.SetValue(coreAnnotationModelKey, model)
	return model
}

func (g coreAnnotationGenerator) GenerateAnnotation(ctx *AnnotationGenerationContext) error {
	value, ok := ctx.Value(coreAnnotationModelKey)
	if !ok {
		return nil
	}
	model, ok := value.(*coreAnnotationModel)
	if !ok {
		return fmt.Errorf("invalid core annotation model")
	}
	if g.propertiesOnly {
		return generateConfigurationProperties(ctx, model)
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
	for _, configuration := range model.Configurations {
		writeGeneratedConfiguration(ctx.buffer(), configuration)
	}
	return nil
}

func generateConfigurationProperties(
	ctx *AnnotationGenerationContext,
	model *coreAnnotationModel,
) error {
	if len(model.ConfigurationProperties) == 0 {
		return nil
	}
	ctx.AddImport("", "goark.dev/goark")
	ctx.AddImport("coreenv", "goark.dev/goark/core/env")
	ctx.AddImport("arkerrors", "goark.dev/goark/errors")
	addConfigurationPropertiesImports(ctx, model.ConfigurationProperties)
	for _, properties := range model.ConfigurationProperties {
		writeConfigurationProperties(ctx.buffer(), properties)
	}
	return nil
}
