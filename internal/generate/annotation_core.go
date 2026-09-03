package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

const coreAnnotationModelKey = "goark.core.annotations"

type coreAnnotationModel struct {
	Configurations          []*annotationConfiguration
	ConfigurationProperties []annotationConfigurationProperties
	Components              []annotationComponent
	UsesValue               bool
	UsesProperties          bool
	configByType            map[string]*annotationConfiguration
}

type annotationConfiguration struct {
	TypeName        string
	Name            string
	Order           int
	Profiles        []string
	PropertySources []annotationPropertySource
	Beans           []annotationBean
	Components      []annotationComponent
	Properties      []annotationConfigurationProperties
	Synthetic       bool
}

type annotationPropertySource struct {
	Location               string
	Name                   string
	Encoding               string
	IgnoreResourceNotFound bool
}

type annotationBean struct {
	Name         string
	MethodName   string
	ReturnType   string
	ReturnsError bool
	Params       []annotationParam
	Options      annotationBeanOptions
	Profiles     []string
	Condition    string
}

type annotationComponent struct {
	Name      string
	TypeName  string
	Fields    []annotationField
	Options   annotationBeanOptions
	Profiles  []string
	Condition string
}

type annotationParam struct {
	Name      string
	Type      string
	Injection injectionSpec
}

type annotationField struct {
	Name      string
	Type      string
	Injection injectionSpec
}

type injectionSpec struct {
	Kind      string
	Qualifier string
	Value     string
	Required  bool
}

type annotationBeanOptions struct {
	Primary                       bool
	Lazy                          bool
	Scope                         string
	DependsOn                     []string
	FactoryDependencies           []string
	InjectionDependencies         []string
	OptionalInjectionDependencies []string
	Order                         *int
	Priority                      *int
}

type annotationDependencyCandidate struct {
	Name     string
	Type     string
	Primary  bool
	Priority *int
}

type annotationDependencyResolver struct {
	byType map[string][]annotationDependencyCandidate
}

type coreAnnotationBinder struct{}

type coreAnnotationGenerator struct{}

func defaultAnnotationExtensions() []AnnotationExtension {
	return []AnnotationExtension{
		{
			Descriptors: coreAnnotationDescriptors(),
			Binder:      coreAnnotationBinder{},
			Generator:   coreAnnotationGenerator{},
		},
		webAnnotationExtension(),
		mvcAnnotationExtension(),
	}
}

func coreAnnotationDescriptors() []AnnotationDescriptor {
	return []AnnotationDescriptor{
		{Name: "configuration", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateCoreNamedStructTypeAnnotation},
		{Name: "component", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateCoreNamedStructTypeAnnotation},
		{Name: "service", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateCoreNamedStructTypeAnnotation},
		{Name: "repository", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateCoreNamedStructTypeAnnotation},
		{Name: "bean", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateCoreBeanAnnotation},
		{Name: "autowired", Targets: []AnnotationTarget{AnnotationTargetField, AnnotationTargetMethod}, Validate: validateCoreInjectionAnnotation},
		{Name: "inject", Targets: []AnnotationTarget{AnnotationTargetField, AnnotationTargetMethod}, Validate: validateCoreInjectionAnnotation},
		{Name: "resource", Targets: []AnnotationTarget{AnnotationTargetField, AnnotationTargetMethod}, Validate: validateCoreInjectionAnnotation},
		{Name: "qualifier", Targets: []AnnotationTarget{AnnotationTargetField, AnnotationTargetMethod}, Validate: validateCoreInjectionAnnotation},
		{Name: "named", Targets: []AnnotationTarget{AnnotationTargetField, AnnotationTargetMethod}, Validate: validateCoreInjectionAnnotation},
		{Name: "value", Targets: []AnnotationTarget{AnnotationTargetField, AnnotationTargetMethod}, Validate: validateCoreInjectionAnnotation},
		{Name: "primary", Targets: []AnnotationTarget{AnnotationTargetType, AnnotationTargetMethod}, Validate: validateCoreBeanOptionAnnotation},
		{Name: "lazy", Targets: []AnnotationTarget{AnnotationTargetType, AnnotationTargetMethod}, Validate: validateCoreLazyAnnotation},
		{Name: "scope", Targets: []AnnotationTarget{AnnotationTargetType, AnnotationTargetMethod}, Validate: validateCoreScopeAnnotation},
		{Name: "depends-on", Targets: []AnnotationTarget{AnnotationTargetType, AnnotationTargetMethod}, Validate: validateCoreDependsOnAnnotation},
		{Name: "order", Targets: []AnnotationTarget{AnnotationTargetType, AnnotationTargetMethod}, Validate: validateCoreOrderAnnotation},
		{Name: "priority", Targets: []AnnotationTarget{AnnotationTargetType, AnnotationTargetMethod}, Validate: validateCorePriorityAnnotation},
		{Name: "profile", Targets: []AnnotationTarget{AnnotationTargetType, AnnotationTargetMethod}, Validate: validateCoreProfileAnnotation},
		{Name: "property-source", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateCorePropertySourceAnnotation},
		{Name: "property-sources", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateCorePropertySourcesAnnotation},
		{Name: "configuration-properties", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateConfigurationPropertiesAnnotation},
		{Name: "conditional", Targets: []AnnotationTarget{AnnotationTargetType, AnnotationTargetMethod}, Validate: validateCoreConditionalAnnotation},
	}
}

func validateCoreStructTypeAnnotation(ctx AnnotationValidationContext) error {
	typeSpec := ctx.Item.TypeSpec()
	if typeSpec == nil {
		return fmt.Errorf("annotation %q requires type target", ctx.Annotation.Name)
	}
	if _, ok := typeSpec.Type.(*ast.StructType); !ok {
		return fmt.Errorf("annotation %q requires struct type target", ctx.Annotation.Name)
	}
	return nil
}

func validateCoreNamedStructTypeAnnotation(ctx AnnotationValidationContext) error {
	if err := validateCoreStructTypeAnnotation(ctx); err != nil {
		return err
	}
	return validateCoreNameAnnotation(ctx.Annotation)
}

func validateCoreBeanAnnotation(ctx AnnotationValidationContext) error {
	if ctx.Item.FuncDecl() == nil || ctx.Item.FuncDecl().Recv == nil {
		return fmt.Errorf("annotation %q requires concrete method with receiver", ctx.Annotation.Name)
	}
	if ctx.Item.ReceiverTypeName() == "" {
		return fmt.Errorf("annotation %q receiver is not supported", ctx.Annotation.Name)
	}
	return validateCoreNameAnnotation(ctx.Annotation)
}

func validateCoreInjectionAnnotation(ctx AnnotationValidationContext) error {
	switch ctx.Target {
	case AnnotationTargetField:
		if len(ctx.Item.Names()) == 0 {
			return fmt.Errorf("annotation %q requires named field target", ctx.Annotation.Name)
		}
	case AnnotationTargetMethod:
		if !ctx.Item.HasAnnotation("bean") {
			return fmt.Errorf("annotation %q requires bean method target", ctx.Annotation.Name)
		}
		if strings.TrimSpace(ctx.Annotation.Selector) == "" {
			return fmt.Errorf("annotation %q on method target requires parameter selector", ctx.Annotation.Name)
		}
		if !methodHasParameter(ctx.Item.FuncDecl(), normalizeSelector(ctx.Annotation.Selector)) {
			return fmt.Errorf("annotation %q selector %q does not match any method parameter", ctx.Annotation.Name, ctx.Annotation.Selector)
		}
	}
	switch ctx.Annotation.Name {
	case "autowired":
		if err := validateBoolArg(ctx.Annotation, "required"); err != nil {
			return err
		}
	case "qualifier", "named", "value":
		return requireAnnotationValue(ctx.Annotation)
	}
	return nil
}

func validateCoreBeanOptionAnnotation(ctx AnnotationValidationContext) error {
	return validateCoreComponentOrBeanOwner(ctx)
}

func validateCoreLazyAnnotation(ctx AnnotationValidationContext) error {
	if err := validateCoreComponentOrBeanOwner(ctx); err != nil {
		return err
	}
	if err := validateAtMostOneAnnotationValue(ctx.Annotation); err != nil {
		return err
	}
	return validateBoolArg(ctx.Annotation, "value")
}

func validateCoreScopeAnnotation(ctx AnnotationValidationContext) error {
	if err := validateCoreComponentOrBeanOwner(ctx); err != nil {
		return err
	}
	value, err := requireAnnotationValueText(ctx.Annotation)
	if err != nil {
		return err
	}
	switch value {
	case ScopeSingleton, ScopePrototype:
		return nil
	default:
		return fmt.Errorf("annotation %q has unsupported scope %q", ctx.Annotation.Name, value)
	}
}

func validateCoreDependsOnAnnotation(ctx AnnotationValidationContext) error {
	if err := validateCoreComponentOrBeanOwner(ctx); err != nil {
		return err
	}
	values, err := requireAnnotationValueTexts(ctx.Annotation)
	if err != nil {
		return err
	}
	for _, value := range values {
		for _, dependency := range strings.Split(value, ",") {
			if strings.TrimSpace(dependency) == "" {
				return fmt.Errorf("annotation %q has empty dependency name", ctx.Annotation.Name)
			}
		}
	}
	return nil
}

func validateCoreOrderAnnotation(ctx AnnotationValidationContext) error {
	if err := validateCoreConfigurationComponentOrBeanOwner(ctx); err != nil {
		return err
	}
	return validateIntValue(ctx.Annotation)
}

func validateCorePriorityAnnotation(ctx AnnotationValidationContext) error {
	if err := validateCoreComponentOrBeanOwner(ctx); err != nil {
		return err
	}
	return validateIntValue(ctx.Annotation)
}

func validateCoreProfileAnnotation(ctx AnnotationValidationContext) error {
	if err := validateCoreConfigurationComponentOrBeanOwner(ctx); err != nil {
		return err
	}
	return requireAnnotationValue(ctx.Annotation)
}

func validateCoreConditionalAnnotation(ctx AnnotationValidationContext) error {
	if err := validateCoreConfigurationComponentOrBeanOwner(ctx); err != nil {
		return err
	}
	return requireAnnotationValue(ctx.Annotation)
}

func validateCorePropertySourceAnnotation(ctx AnnotationValidationContext) error {
	if !ctx.Item.HasAnnotation("configuration") {
		return fmt.Errorf("annotation %q requires configuration type target", ctx.Annotation.Name)
	}
	return requireAnnotationValue(ctx.Annotation)
}

func validateCorePropertySourcesAnnotation(ctx AnnotationValidationContext) error {
	if !ctx.Item.HasAnnotation("configuration") {
		return fmt.Errorf("annotation %q requires configuration type target", ctx.Annotation.Name)
	}
	return requireAnnotationValue(ctx.Annotation)
}

func validateCoreComponentOrBeanOwner(ctx AnnotationValidationContext) error {
	switch ctx.Target {
	case AnnotationTargetType:
		if componentOptionKind(ctx.Item.annotations) == "" {
			return fmt.Errorf("annotation %q requires component type target", ctx.Annotation.Name)
		}
	case AnnotationTargetMethod:
		if !ctx.Item.HasAnnotation("bean") {
			return fmt.Errorf("annotation %q requires bean method target", ctx.Annotation.Name)
		}
	}
	return nil
}

func validateCoreConfigurationComponentOrBeanOwner(ctx AnnotationValidationContext) error {
	switch ctx.Target {
	case AnnotationTargetType:
		if !ctx.Item.HasAnnotation("configuration") && componentOptionKind(ctx.Item.annotations) == "" {
			return fmt.Errorf("annotation %q requires configuration or component type target", ctx.Annotation.Name)
		}
	case AnnotationTargetMethod:
		if !ctx.Item.HasAnnotation("bean") {
			return fmt.Errorf("annotation %q requires bean method target", ctx.Annotation.Name)
		}
	}
	return nil
}

func requireAnnotationValue(annotation Annotation) error {
	_, err := requireAnnotationValueText(annotation)
	return err
}

func requireAnnotationValueText(annotation Annotation) (string, error) {
	values, err := requireAnnotationValueTexts(annotation)
	if err != nil {
		return "", err
	}
	if len(values) > 1 {
		return "", fmt.Errorf("annotation %q accepts exactly one value argument", annotation.Name)
	}
	return values[0], nil
}

func requireAnnotationValueTexts(annotation Annotation) ([]string, error) {
	values := annotationValueTexts(annotation)
	if len(values) == 0 {
		return nil, fmt.Errorf("annotation %q requires value argument", annotation.Name)
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("annotation %q requires value argument", annotation.Name)
		}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func validateAtMostOneAnnotationValue(annotation Annotation) error {
	if len(annotation.Values) > 1 {
		return fmt.Errorf("annotation %q accepts at most one value argument", annotation.Name)
	}
	return nil
}

func validateCoreNameAnnotation(annotation Annotation) error {
	if err := validateAtMostOneAnnotationValue(annotation); err != nil {
		return err
	}
	if _, hasName := annotation.Args["name"]; hasName {
		if _, hasValue := annotation.Args["value"]; hasValue {
			return fmt.Errorf("annotation %q accepts either name or value argument", annotation.Name)
		}
	}
	return nil
}

func validateIntValue(annotation Annotation) error {
	value, err := requireAnnotationValueText(annotation)
	if err != nil {
		return err
	}
	if _, err := strconv.Atoi(value); err != nil {
		return fmt.Errorf("annotation %q requires integer value: %w", annotation.Name, err)
	}
	return nil
}

func validateBoolArg(annotation Annotation, key string) error {
	value, ok := annotation.Args[key]
	if !ok || strings.TrimSpace(value.text) == "" {
		return nil
	}
	if _, err := strconv.ParseBool(strings.TrimSpace(value.text)); err != nil {
		return fmt.Errorf("annotation %q argument %q requires boolean value: %w", annotation.Name, key, err)
	}
	return nil
}

func normalizeSelector(selector string) string {
	selector = strings.TrimSpace(selector)
	if strings.HasPrefix(selector, "param=") {
		selector = strings.TrimSpace(strings.TrimPrefix(selector, "param="))
	}
	return strings.Trim(selector, "\"")
}

func methodHasParameter(fn *ast.FuncDecl, name string) bool {
	if fn == nil || fn.Type.Params == nil || name == "" {
		return false
	}
	for index, field := range fn.Type.Params.List {
		if len(field.Names) == 0 {
			if name == fmt.Sprintf("arg%d", index) {
				return true
			}
			continue
		}
		for _, paramName := range field.Names {
			if paramName.Name == name {
				return true
			}
		}
	}
	return false
}

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
			name = ctx.PackageName()
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

func writeGeneratedConfiguration(builder *bytes.Buffer, configuration *annotationConfiguration) {
	if configuration.Synthetic {
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
	builder.WriteString(") ConfigureEnvironment(ctx context.Context, environment coreenv.ConfigurableEnvironment) error {\n")
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
		builder.WriteString(")\nif err != nil {\nreturn err\n}\nif source != nil {\nif err := environment.PropertySources().AddLast(source); err != nil {\nreturn err\n}\n}\n")
	}
	builder.WriteString("return nil\n}\n\n")
}

func writeRegisterWithContext(builder *bytes.Buffer, configuration *annotationConfiguration) {
	builder.WriteString("func (c ")
	builder.WriteString(configuration.TypeName)
	builder.WriteString(") Register(ctx context.Context, registry *container.Registry) error {\n")
	builder.WriteString("return c.RegisterWithContext(ctx, goark.NewConfigurationContext(nil, registry))\n")
	builder.WriteString("}\n\n")
	builder.WriteString("func (c ")
	builder.WriteString(configuration.TypeName)
	builder.WriteString(") RegisterWithContext(ctx context.Context, config goark.ConfigurationContext) error {\n")
	if len(configuration.Profiles) > 0 {
		writeProfileGuard(builder, strings.Join(wrapExpressions(configuration.Profiles), " | "), configuration.Name, "return nil")
	}
	if len(configuration.Components) > 0 || len(configuration.Beans) > 0 || len(configuration.Properties) > 0 {
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

func writeComponentRegistration(builder *bytes.Buffer, component annotationComponent) {
	writeConditionalStart(builder, component.Name, component.Profiles, component.Condition)
	builder.WriteString("if err := container.Register(registry, ")
	builder.WriteString(strconv.Quote(component.Name))
	builder.WriteString(", func(ctx context.Context, resolver container.Resolver) (out *")
	builder.WriteString(component.TypeName)
	builder.WriteString(", err error) {\nout = &")
	builder.WriteString(component.TypeName)
	builder.WriteString("{}\nreturn out, nil\n}")
	writeContainerOptions(builder, component.Options)
	writeComponentDependencyInjector(builder, component)
	builder.WriteString("); err != nil {\nreturn err\n}\n")
	writeConditionalEnd(builder, component.Profiles, component.Condition)
}

func writeBeanRegistrationFromAnnotation(builder *bytes.Buffer, bean annotationBean) {
	writeConditionalStart(builder, bean.Name, bean.Profiles, bean.Condition)
	builder.WriteString("if err := container.Register(registry, ")
	builder.WriteString(strconv.Quote(bean.Name))
	builder.WriteString(", func(ctx context.Context, resolver container.Resolver) (out ")
	builder.WriteString(bean.ReturnType)
	builder.WriteString(", err error) {\n")
	args := make([]string, 0, len(bean.Params))
	for _, param := range bean.Params {
		args = append(args, param.Name)
		writeParamResolution(builder, param)
	}
	if bean.ReturnsError {
		builder.WriteString("return c.")
		builder.WriteString(bean.MethodName)
		builder.WriteString("(")
		builder.WriteString(strings.Join(args, ", "))
		builder.WriteString(")\n")
	} else {
		builder.WriteString("out = c.")
		builder.WriteString(bean.MethodName)
		builder.WriteString("(")
		builder.WriteString(strings.Join(args, ", "))
		builder.WriteString(")\nreturn out, nil\n")
	}
	builder.WriteString("}")
	writeContainerOptions(builder, bean.Options)
	builder.WriteString("); err != nil {\nreturn err\n}\n")
	writeConditionalEnd(builder, bean.Profiles, bean.Condition)
}

func writeParamResolution(builder *bytes.Buffer, param annotationParam) {
	builder.WriteString("var ")
	builder.WriteString(param.Name)
	builder.WriteByte(' ')
	builder.WriteString(param.Type)
	builder.WriteByte('\n')
	writeInjectionAssignment(builder, param.Name, param.Type, param.Injection, "return out, err")
}

func writeComponentDependencyInjector(builder *bytes.Buffer, component annotationComponent) {
	if len(component.Fields) == 0 {
		return
	}
	builder.WriteString(", container.WithTypedDependencyInjector(func(ctx context.Context, resolver container.Resolver, out *")
	builder.WriteString(component.TypeName)
	builder.WriteString(") error {\n")
	builder.WriteString("var err error\n")
	for _, field := range component.Fields {
		writeInjectionAssignment(builder, "out."+field.Name, field.Type, field.Injection, "return err")
	}
	builder.WriteString("return nil\n})")
}

func writeInjectionAssignment(builder *bytes.Buffer, target string, typ string, injection injectionSpec, errorReturn string) {
	if injection.Kind == "value" {
		builder.WriteString(target)
		builder.WriteString(", err = goark.ResolveValueAs[")
		builder.WriteString(typ)
		builder.WriteString("](config.Environment(), ")
		builder.WriteString(strconv.Quote(injection.Value))
		builder.WriteString(")\n")
		writeInjectionErrorCheck(builder, true, errorReturn)
		return
	}
	if injection.Qualifier != "" {
		builder.WriteString(target)
		builder.WriteString(", err = container.GetByType[")
		builder.WriteString(typ)
		builder.WriteString("](ctx, resolver, container.WithQualifier(")
		builder.WriteString(strconv.Quote(injection.Qualifier))
		builder.WriteString("))\n")
		writeInjectionErrorCheck(builder, injection.Required, errorReturn)
		return
	}
	builder.WriteString(target)
	builder.WriteString(", err = container.GetByType[")
	builder.WriteString(typ)
	builder.WriteString("](ctx, resolver)\n")
	writeInjectionErrorCheck(builder, injection.Required, errorReturn)
}

func writeInjectionErrorCheck(builder *bytes.Buffer, required bool, errorReturn string) {
	builder.WriteString("if err != nil {\n")
	if required {
		builder.WriteString(errorReturn)
		builder.WriteByte('\n')
	} else {
		builder.WriteString("if !arkerrors.Is(err, arkerrors.CodeNotFound) {\n")
		builder.WriteString(errorReturn)
		builder.WriteString("\n}\n")
	}
	builder.WriteString("}\n")
}

func writeConditionalStart(builder *bytes.Buffer, name string, profiles []string, condition string) {
	if len(profiles) > 0 {
		writeProfileGuard(builder, strings.Join(wrapExpressions(profiles), " | "), name, "")
	}
	if condition != "" {
		builder.WriteString("if matched, err := (")
		builder.WriteString(condition)
		builder.WriteString("{}).Matches(config, goark.AnnotationMetadata{Name: ")
		builder.WriteString(strconv.Quote(name))
		builder.WriteString("}); err != nil {\nreturn err\n} else if matched {\n")
	}
}

func writeConditionalEnd(builder *bytes.Buffer, profiles []string, condition string) {
	if condition != "" {
		builder.WriteString("}\n")
	}
	if len(profiles) > 0 {
		builder.WriteString("}\n")
	}
}

func writeProfileGuard(builder *bytes.Buffer, expression string, name string, unmatchedAction string) {
	builder.WriteString("if matched, err := (goark.ProfileCondition{Expression: ")
	builder.WriteString(strconv.Quote(expression))
	builder.WriteString("}).Matches(config, goark.AnnotationMetadata{Name: ")
	builder.WriteString(strconv.Quote(name))
	builder.WriteString("}); err != nil {\nreturn err\n} else if !matched {\n")
	if unmatchedAction != "" {
		builder.WriteString(unmatchedAction)
		builder.WriteByte('\n')
	} else {
		builder.WriteString("} else {\n")
		return
	}
	builder.WriteString("}\n")
}

func writeContainerOptions(builder *bytes.Buffer, options annotationBeanOptions) {
	for _, option := range containerOptions(options) {
		builder.WriteString(", ")
		builder.WriteString(option)
	}
}

func containerOptions(options annotationBeanOptions) []string {
	out := make([]string, 0, 9)
	if options.Primary {
		out = append(out, "container.WithPrimary()")
	}
	if options.Lazy {
		out = append(out, "container.WithLazy()")
	}
	if options.Scope != "" {
		switch options.Scope {
		case ScopePrototype:
			out = append(out, "container.WithPrototype()")
		case ScopeSingleton:
			out = append(out, "container.WithSingleton()")
		default:
			out = append(out, "container.WithScope(container.Scope("+strconv.Quote(options.Scope)+"))")
		}
	}
	if len(options.DependsOn) > 0 {
		out = append(out, dependencyOption("container.WithDependsOn", options.DependsOn))
	}
	if options.Order != nil {
		out = append(out, "container.WithOrder("+strconv.Itoa(*options.Order)+")")
	}
	if options.Priority != nil {
		out = append(out, "container.WithPriority("+strconv.Itoa(*options.Priority)+")")
	}
	if len(options.FactoryDependencies) > 0 {
		out = append(out, dependencyOption("container.WithFactoryDependencies", options.FactoryDependencies))
	}
	if len(options.InjectionDependencies) > 0 {
		out = append(out, dependencyOption("container.WithInjectionDependencies", options.InjectionDependencies))
	}
	if len(options.OptionalInjectionDependencies) > 0 {
		out = append(out, dependencyOption("container.WithOptionalInjectionDependencies", options.OptionalInjectionDependencies))
	}
	return out
}

func dependencyOption(function string, names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, strconv.Quote(name))
	}
	return function + "(" + strings.Join(quoted, ", ") + ")"
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
	if hasAnnotation(annotations, "inject") || hasAnnotation(annotations, "autowired") || qualifier != "" {
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
			return value.text
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
