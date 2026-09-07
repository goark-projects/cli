// Package application 提供 Goark Boot 应用入口注解扩展。
package application

import (
	"fmt"
	"go/ast"
	"strconv"

	"goark.dev/cli/internal/generate"
)

const modelKey = "goark.application.annotation"

type model struct {
	typeName string
	web      bool
}

type binder struct{}

type generator struct{}

// Extension 返回应用入口注解的描述、绑定和生成实现。
func Extension() generate.AnnotationExtension {
	return generate.AnnotationExtension{
		Name: "application",
		Descriptors: []generate.AnnotationDescriptor{{
			Name:     "application",
			Targets:  []generate.AnnotationTarget{generate.AnnotationTargetType},
			Validate: validate,
		}},
		Binder:    binder{},
		Generator: generator{},
	}
}

func validate(ctx generate.AnnotationValidationContext) error {
	item := ctx.Item
	typeSpec := item.TypeSpec()
	if typeSpec == nil || typeSpec.Name == nil {
		return fmt.Errorf("application annotation requires named type")
	}
	if _, ok := typeSpec.Type.(*ast.StructType); !ok {
		return fmt.Errorf("application type %s must be a struct", item.TypeName())
	}
	if !item.HasAnnotation("configuration") {
		return fmt.Errorf("application type %s must also declare configuration", item.TypeName())
	}
	annotation := ctx.Annotation
	if annotation.Selector != "" || len(annotation.Values) > 0 {
		return fmt.Errorf("application annotation only supports named web argument")
	}
	for name, value := range annotation.Args {
		if name != "web" {
			return fmt.Errorf("application annotation does not support argument %q", name)
		}
		if _, err := strconv.ParseBool(value.Text()); err != nil {
			return fmt.Errorf("application web argument must be true or false: %w", err)
		}
	}
	return nil
}

func (binder) BindAnnotation(
	ctx *generate.AnnotationBindingContext,
	item generate.AnnotationItem,
) error {
	if !item.HasAnnotation("application") {
		return nil
	}
	if _, exists := ctx.Value(modelKey); exists {
		return fmt.Errorf("multiple application annotations are not supported")
	}
	entry := &model{typeName: item.TypeName()}
	count := 0
	for _, annotation := range item.Annotations() {
		if annotation.Name != "application" {
			continue
		}
		count++
		if count > 1 {
			return fmt.Errorf("multiple application annotations are not supported")
		}
		if value, ok := annotation.Args["web"]; ok {
			entry.web, _ = strconv.ParseBool(value.Text())
		}
	}
	ctx.SetValue(modelKey, entry)
	return nil
}

func (generator) GenerateAnnotation(ctx *generate.AnnotationGenerationContext) error {
	value, exists := ctx.Value(modelKey)
	if !exists {
		return nil
	}
	entry, ok := value.(*model)
	if !ok || entry == nil {
		return fmt.Errorf("invalid application annotation model")
	}
	value, _ = ctx.Value(generate.ConfigurationTypesModelKey)
	configurationTypes, _ := value.([]string)
	if len(configurationTypes) == 0 {
		return fmt.Errorf("application %s has no generated configuration", entry.typeName)
	}
	addImports(ctx, entry.web)
	writeRun(ctx, entry.web, configurationTypes)
	return nil
}
