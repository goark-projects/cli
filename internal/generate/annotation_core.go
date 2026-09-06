package generate

import (
	"fmt"
	"go/ast"
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
	SourceTypeName  string
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
	Name              string
	MethodName        string
	ReturnType        string
	ReturnsError      bool
	Params            []annotationParam
	Options           annotationBeanOptions
	Profiles          []string
	Condition         string
	ConfigurationType string
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
			Name:        "core",
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
