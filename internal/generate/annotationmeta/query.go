// Package annotationmeta 提供与具体生成器无关的注解元数据查询。
package annotationmeta

import (
	"strings"

	"goark.dev/cli/internal/generate/annotationparse"
)

// Annotation 是稳定的注解语法模型。
type Annotation = annotationparse.Annotation

// Has 判断注解集合是否包含指定名称。
func Has(annotations []Annotation, name string) bool {
	for _, annotation := range annotations {
		if annotation.Name == name {
			return true
		}
	}
	return false
}

// ValueTexts 返回注解的全部位置值或命名 value。
func ValueTexts(annotation Annotation) []string {
	if len(annotation.Values) > 0 {
		values := make([]string, 0, len(annotation.Values))
		for _, value := range annotation.Values {
			values = append(values, value.Text())
		}
		return values
	}
	if value, ok := annotation.Args["value"]; ok {
		return []string{value.Text()}
	}
	return nil
}

// String 返回指定注解的第一个名称或值参数。
func String(annotations []Annotation, name, fallback string) string {
	for _, annotation := range annotations {
		if annotation.Name != name {
			continue
		}
		for _, key := range []string{"name", "value"} {
			if value, ok := annotation.Args[key]; ok {
				return value.Text()
			}
		}
	}
	return fallback
}

// Strings 返回同名注解的全部非空值。
func Strings(annotations []Annotation, name string) []string {
	values := make([]string, 0)
	for _, annotation := range annotations {
		if annotation.Name != name {
			continue
		}
		for _, value := range ValueTexts(annotation) {
			if value != "" {
				values = append(values, value)
			}
		}
	}
	return values
}

// BySelector 按规范化选择器分组注解。
func BySelector(annotations []Annotation) map[string][]Annotation {
	out := make(map[string][]Annotation)
	for _, annotation := range annotations {
		selector := annotation.Selector
		if strings.HasPrefix(selector, "param=") {
			selector = strings.Trim(strings.TrimPrefix(selector, "param="), "\"")
		}
		if selector != "" {
			out[selector] = append(out[selector], annotation)
		}
	}
	return out
}
