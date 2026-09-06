package generate

import (
	"fmt"
	"go/ast"
	"strconv"
	"strings"
)

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
