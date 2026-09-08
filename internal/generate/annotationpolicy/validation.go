package annotationpolicy

import (
	"fmt"
	"go/ast"
	"goark.dev/cli/internal/generate/annotationparse"
	"slices"
	"strings"
)

// ValidateConflicts 验证内置注解中会互相覆盖的角色和注入设置。
func ValidateConflicts(items []Annotation) error {
	roles := []string{
		"configuration", "component", "service", "repository", "goark-web:controller",
		"goark-web:rest-controller", "goark-web:mvc-controller", "goark-web:controller-advice",
		"goark-web:rest-controller-advice", "goark-web:web-filter", "goark-web:web-interceptor",
	}
	var selected []string
	for _, item := range items {
		if slices.Contains(roles, item.Name) {
			selected = append(selected, item.Name)
		}
	}
	if len(selected) > 1 {
		return fmt.Errorf("conflicting component annotations: %s", strings.Join(selected, ", "))
	}
	groups := make(map[string][]Annotation)
	var keys []string
	for _, item := range items {
		if !IsInjection(item.Name) {
			continue
		}
		key := SelectorKey(item)
		if _, exists := groups[key]; !exists {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], item)
	}
	for _, key := range keys {
		group := groups[key]
		present := make(map[string]bool)
		for _, item := range group {
			present[item.Name] = true
		}
		conflict := present["value"] && len(group) > 1 || present["resource"] && len(group) > 1 ||
			present["inject"] && present["autowired"] || present["qualifier"] && present["named"]
		for _, item := range group {
			if item.Name == "autowired" && item.Args["qualifier"].Text() != "" &&
				(present["qualifier"] || present["named"]) {
				conflict = true
			}
		}
		if conflict {
			return fmt.Errorf("conflicting injection annotations for parameter/field %q", key)
		}
	}
	return nil
}

// IsInjection 判断核心注入注解。
func IsInjection(name string) bool {
	return slices.Contains([]string{
		"autowired", "inject", "resource", "qualifier", "named", "value",
	}, name)
}

// SelectorKey 规范化参数选择器的可选 param= 前缀及引号。
func SelectorKey(item Annotation) string {
	selector := strings.TrimPrefix(strings.TrimSpace(item.Selector), "param=")
	return strings.Trim(strings.TrimSpace(selector), "\"")
}

// ValidateFieldOwner 防止核心注入注解被写在生成器不会装配的类型上。
func ValidateFieldOwner(items []Annotation, decl *ast.GenDecl, typ *ast.TypeSpec) error {
	injection := ""
	for _, item := range items {
		if !IsInjection(item.Name) {
			continue
		}
		injection = item.Name
		if item.Selector != "" {
			return fmt.Errorf("%s: field annotation %s does not accept a selector",
				typ.Name.Name, item.Name)
		}
	}
	if injection == "" {
		return nil
	}
	owners, err := (annotationparse.Namespaces{"goark", "goark-web"}).ParseGroups(
		decl.Doc, typ.Doc, typ.Comment,
	)
	if err != nil {
		return err
	}
	for _, owner := range owners {
		if slices.Contains([]string{
			"component", "service", "repository", "goark-web:controller",
			"goark-web:rest-controller", "goark-web:mvc-controller",
			"goark-web:controller-advice", "goark-web:rest-controller-advice",
			"goark-web:web-filter", "goark-web:web-interceptor",
		}, owner.Name) {
			return nil
		}
	}
	return fmt.Errorf("%s: field annotation %s requires a component owner",
		typ.Name.Name, injection)
}
