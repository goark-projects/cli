package generate

import (
	"bytes"
	"strconv"
	"strings"
)

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
