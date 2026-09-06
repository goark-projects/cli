package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"strconv"
	"strings"
)

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
		builder.WriteString("return ")
		writeConfigurationMethodReceiver(builder, bean)
		builder.WriteString(bean.MethodName)
		builder.WriteString("(")
		builder.WriteString(strings.Join(args, ", "))
		builder.WriteString(")\n")
	} else {
		builder.WriteString("out = ")
		writeConfigurationMethodReceiver(builder, bean)
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

func writeConfigurationMethodReceiver(builder *bytes.Buffer, bean annotationBean) {
	if bean.ConfigurationType != "" {
		builder.WriteString("c.source.")
		return
	}
	builder.WriteString("c.")
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
	builder.WriteString(", container.WithTypedDependencyInjector(")
	builder.WriteString("func(ctx context.Context, resolver container.Resolver, out *")
	builder.WriteString(component.TypeName)
	builder.WriteString(") error {\n")
	builder.WriteString("var err error\n")
	for _, field := range component.Fields {
		writeInjectionAssignment(builder, "out."+field.Name, field.Type, field.Injection, "return err")
	}
	builder.WriteString("return nil\n})")
}

func writeInjectionAssignment(
	builder *bytes.Buffer,
	target string,
	typ string,
	injection injectionSpec,
	errorReturn string,
) {
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

func writeConditionalStart(
	builder *bytes.Buffer,
	name string,
	profiles []string,
	condition string,
) {
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

func writeProfileGuard(
	builder *bytes.Buffer,
	expression string,
	name string,
	unmatchedAction string,
) {
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
		out = append(out,
			dependencyOption("container.WithFactoryDependencies", options.FactoryDependencies))
	}
	if len(options.InjectionDependencies) > 0 {
		out = append(out,
			dependencyOption("container.WithInjectionDependencies", options.InjectionDependencies))
	}
	if len(options.OptionalInjectionDependencies) > 0 {
		out = append(out, dependencyOption(
			"container.WithOptionalInjectionDependencies",
			options.OptionalInjectionDependencies,
		))
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
		return "", annotationError("accepts exactly one value argument", annotation.Name)
	}
	return values[0], nil
}

func requireAnnotationValueTexts(annotation Annotation) ([]string, error) {
	values := annotationValueTexts(annotation)
	if len(values) == 0 {
		return nil, annotationError("requires value argument", annotation.Name)
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, annotationError("requires value argument", annotation.Name)
		}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func validateAtMostOneAnnotationValue(annotation Annotation) error {
	if len(annotation.Values) > 1 {
		return annotationError("accepts at most one value argument", annotation.Name)
	}
	return nil
}

func validateCoreNameAnnotation(annotation Annotation) error {
	if err := validateAtMostOneAnnotationValue(annotation); err != nil {
		return err
	}
	if _, hasName := annotation.Args["name"]; hasName {
		if _, hasValue := annotation.Args["value"]; hasValue {
			return annotationError("accepts either name or value argument", annotation.Name)
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
		return annotationError("requires integer value: %w", annotation.Name, err)
	}
	return nil
}

func validateBoolArg(annotation Annotation, key string) error {
	value, ok := annotation.Args[key]
	if !ok || strings.TrimSpace(value.Text()) == "" {
		return nil
	}
	if _, err := strconv.ParseBool(strings.TrimSpace(value.Text())); err != nil {
		return fmt.Errorf(
			"annotation %q argument %q requires boolean value: %w",
			annotation.Name,
			key,
			err,
		)
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
