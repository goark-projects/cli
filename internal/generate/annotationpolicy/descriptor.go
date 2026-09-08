// Package annotationpolicy 定义注解语义策略，与注释语法解析解耦。
package annotationpolicy

import (
	"fmt"
	"slices"
	"strings"

	"goark.dev/cli/internal/generate/annotationparse"
)

type Annotation = annotationparse.Annotation

// Descriptor 描述注解目标、属性校验与声明基数；类型参数隔离 AST 绑定上下文。
type Descriptor[T comparable, C any] struct {
	Name     string
	Targets  []T
	Validate func(C) error
	// Repeatable 允许同一目标多次声明，扩展必须定义组合语义。
	Repeatable bool
	// InstanceKey 区分不同参数，默认整个节点只能声明一次。
	InstanceKey func(Annotation) string
}

// DefaultDescriptor 创建内置注解的基数策略，扩展可直接声明自己的 Descriptor。
func DefaultDescriptor[T comparable, C any](
	name string, validate func(C) error, targets ...T,
) Descriptor[T, C] {
	descriptor := Descriptor[T, C]{Name: name, Targets: targets, Validate: validate}
	descriptor.Repeatable = slices.Contains([]string{
		"property-source", "depends-on", "profile",
	}, name)
	if IsInjection(name) {
		descriptor.InstanceKey = SelectorKey
	} else if slices.Contains([]string{
		"goark-web:path-variable", "goark-web:request-param", "goark-web:request-header",
		"goark-web:cookie-value", "goark-web:model-attribute", "goark-web:request-attribute",
		"goark-web:session-attribute", "goark-web:matrix-variable", "goark-web:request-part",
	}, name) {
		descriptor.InstanceKey = func(item Annotation) string {
			if key := SelectorKey(item); key != "" {
				return key
			}
			return strings.TrimSpace(item.Args["param"].Text())
		}
	}
	return descriptor
}

// ValidateDeclarations 校验重复声明与互斥组合，未知注解由分发器单独报告。
func ValidateDeclarations[T comparable, C any](
	items []Annotation, descriptors map[string]Descriptor[T, C],
) error {
	seen := make(map[string]bool)
	for _, item := range items {
		if strings.HasPrefix(item.Name, "goark-web:") {
			selector, param := SelectorKey(item), strings.TrimSpace(item.Args["param"].Text())
			if selector != "" && param != "" && selector != param {
				return fmt.Errorf("%s has conflicting parameter selectors %q and %q",
					item.Name, selector, param)
			}
		}
		descriptor := descriptors[item.Name]
		key := item.Name
		if descriptor.InstanceKey != nil {
			key += "\x00" + descriptor.InstanceKey(item)
		}
		if seen[key] && !descriptor.Repeatable {
			return fmt.Errorf("multiple %s annotations on the same target",
				strings.TrimPrefix(item.Name, "goark-web:"))
		}
		seen[key] = true
	}
	return ValidateConflicts(items)
}
