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

type coreAnnotationGenerator struct {
	propertiesOnly bool
}

func defaultAnnotationExtensions() []AnnotationExtension {
	return []AnnotationExtension{
		{
			Name:        "core",
			Descriptors: coreAnnotationDescriptors(),
			Binder:      coreAnnotationBinder{},
			Generator:   coreAnnotationGenerator{},
		},
		{
			Name:      "properties",
			Generator: coreAnnotationGenerator{propertiesOnly: true},
		},
		webAnnotationExtension(),
		mvcAnnotationExtension(),
	}
}

func coreAnnotationDescriptors() []AnnotationDescriptor {
	return []AnnotationDescriptor{
		typeDesc("configuration", validateCoreNamedStructTypeAnnotation),
		typeDesc("component", validateCoreNamedStructTypeAnnotation),
		typeDesc("service", validateCoreNamedStructTypeAnnotation),
		typeDesc("repository", validateCoreNamedStructTypeAnnotation),
		methodDesc("bean", validateCoreBeanAnnotation),
		fieldMethodDesc("autowired", validateCoreInjectionAnnotation),
		fieldMethodDesc("inject", validateCoreInjectionAnnotation),
		fieldMethodDesc("resource", validateCoreInjectionAnnotation),
		fieldMethodDesc("qualifier", validateCoreInjectionAnnotation),
		fieldMethodDesc("named", validateCoreInjectionAnnotation),
		fieldMethodDesc("value", validateCoreInjectionAnnotation),
		typeMethodDesc("primary", validateCoreBeanOptionAnnotation),
		typeMethodDesc("lazy", validateCoreLazyAnnotation),
		typeMethodDesc("scope", validateCoreScopeAnnotation),
		typeMethodDesc("depends-on", validateCoreDependsOnAnnotation),
		typeMethodDesc("order", validateCoreOrderAnnotation),
		typeMethodDesc("priority", validateCorePriorityAnnotation),
		typeMethodDesc("profile", validateCoreProfileAnnotation),
		typeDesc("property-source", validateCorePropertySourceAnnotation),
		typeDesc("property-sources", validateCorePropertySourcesAnnotation),
		typeDesc("configuration-properties", validateConfigurationPropertiesAnnotation),
		typeMethodDesc("conditional", validateCoreConditionalAnnotation),
	}
}

func fieldMethodDesc(name string, validate annotationValidateFunc) AnnotationDescriptor {
	return newAnnotationDesc(name, validate, AnnotationTargetField, AnnotationTargetMethod)
}

func typeMethodDesc(name string, validate annotationValidateFunc) AnnotationDescriptor {
	return newAnnotationDesc(name, validate, AnnotationTargetType, AnnotationTargetMethod)
}

func newAnnotationDesc(
	name string,
	validate annotationValidateFunc,
	targets ...AnnotationTarget,
) AnnotationDescriptor {
	return AnnotationDescriptor{
		Name: name, Targets: targets, Validate: validate,
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
		return fmt.Errorf(
			"annotation %q requires concrete method with receiver",
			ctx.Annotation.Name,
		)
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
			return fmt.Errorf(
				"annotation %q on method target requires parameter selector",
				ctx.Annotation.Name,
			)
		}
		if !methodHasParameter(ctx.Item.FuncDecl(), normalizeSelector(ctx.Annotation.Selector)) {
			return fmt.Errorf(
				"annotation %q selector %q does not match any method parameter",
				ctx.Annotation.Name,
				ctx.Annotation.Selector,
			)
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
		if !ctx.Item.HasAnnotation("configuration") &&
			componentOptionKind(ctx.Item.annotations) == "" {
			return fmt.Errorf(
				"annotation %q requires configuration or component type target",
				ctx.Annotation.Name,
			)
		}
	case AnnotationTargetMethod:
		if !ctx.Item.HasAnnotation("bean") {
			return fmt.Errorf("annotation %q requires bean method target", ctx.Annotation.Name)
		}
	}
	return nil
}
