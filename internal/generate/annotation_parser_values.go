package generate

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func hasAnnotation(annotations []Annotation, name string) bool {
	for _, annotation := range annotations {
		if annotation.Name == name {
			return true
		}
	}
	return false
}

func annotationName(annotations []Annotation, name string, fallback string) string {
	if value := annotationString(annotations, name, ""); value != "" {
		return value
	}
	return fallback
}

func annotationString(annotations []Annotation, name string, fallback string) string {
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

func annotationStrings(annotations []Annotation, name string) []string {
	values := make([]string, 0)
	for _, annotation := range annotations {
		if annotation.Name != name {
			continue
		}
		for _, value := range annotationValueTexts(annotation) {
			if value != "" {
				values = append(values, value)
			}
		}
	}
	return values
}

func annotationValueTexts(annotation Annotation) []string {
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

func annotationInt(annotations []Annotation, name string, fallback int) int {
	for _, annotation := range annotations {
		if annotation.Name != name {
			continue
		}
		if value, ok := annotation.Args["value"]; ok {
			if parsed, err := strconv.Atoi(value.Text()); err == nil {
				return parsed
			}
		}
	}
	return fallback
}

func annotationBool(annotation Annotation, key string, fallback bool) bool {
	value, ok := annotation.Args[key]
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(value.Text())
	if err != nil {
		return fallback
	}
	return parsed
}

func annotationBoolByName(annotations []Annotation, name string, fallback bool) bool {
	for _, annotation := range annotations {
		if annotation.Name == name {
			return annotationBool(annotation, "value", fallback)
		}
	}
	return fallback
}

func argString(annotation Annotation, key string, fallback string) string {
	if value, ok := annotation.Args[key]; ok {
		return value.Text()
	}
	return fallback
}

func annotationsBySelector(annotations []Annotation) map[string][]Annotation {
	out := make(map[string][]Annotation)
	for _, annotation := range annotations {
		selector := annotation.Selector
		if strings.HasPrefix(selector, "param=") {
			selector = strings.Trim(strings.TrimPrefix(selector, "param="), "\"")
		}
		if selector == "" {
			continue
		}
		out[selector] = append(out[selector], annotation)
	}
	return out
}

func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	switch typ := recv.List[0].Type.(type) {
	case *ast.Ident:
		return typ.Name
	case *ast.StarExpr:
		if ident, ok := typ.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	var builder bytes.Buffer
	_ = printer.Fprint(&builder, fset, expr)
	return builder.String()
}

func wrapExpressions(expressions []string) []string {
	out := make([]string, 0, len(expressions))
	for _, expression := range expressions {
		out = append(out, "("+expression+")")
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func lowerCamel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	r, size := utf8.DecodeRuneInString(value)
	if r == utf8.RuneError {
		return value
	}
	return string(unicode.ToLower(r)) + value[size:]
}
