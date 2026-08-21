package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Annotation 表示一条 //goark:* 注解。
type Annotation struct {
	Name     string
	Selector string
	Args     map[string]AnnotationArg
	Values   []AnnotationArg
}

// AnnotationArg 表示注解参数。
type AnnotationArg struct {
	text string
}

// Text 返回注解参数文本值。
func (a AnnotationArg) Text() string {
	return a.text
}

func parseAnnotations(group *ast.CommentGroup) ([]Annotation, error) {
	if group == nil {
		return nil, nil
	}
	annotations := make([]Annotation, 0)
	for _, comment := range group.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		if !strings.HasPrefix(text, "goark:") {
			continue
		}
		annotation, err := parseAnnotation(strings.TrimPrefix(text, "goark:"))
		if err != nil {
			return nil, err
		}
		annotations = append(annotations, annotation)
	}
	return annotations, nil
}

func parseAnnotation(raw string) (Annotation, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Annotation{}, fmt.Errorf("annotation name is required")
	}
	nameEnd := strings.IndexAny(raw, "([ \t")
	if nameEnd < 0 {
		return Annotation{Name: raw, Args: map[string]AnnotationArg{}}, nil
	}
	annotation := Annotation{Name: strings.TrimSpace(raw[:nameEnd]), Args: map[string]AnnotationArg{}}
	if annotation.Name == "" {
		return Annotation{}, fmt.Errorf("annotation name is required")
	}
	rest := strings.TrimSpace(raw[nameEnd:])
	if strings.HasPrefix(rest, "[") {
		end := strings.Index(rest, "]")
		if end < 0 {
			return Annotation{}, fmt.Errorf("annotation %q selector is not closed", annotation.Name)
		}
		annotation.Selector = strings.TrimSpace(rest[1:end])
		rest = strings.TrimSpace(rest[end+1:])
	}
	if strings.HasPrefix(rest, "(") {
		if !strings.HasSuffix(rest, ")") {
			return Annotation{}, fmt.Errorf("annotation %q arguments are not closed", annotation.Name)
		}
		args, values, err := parseAnnotationArgs(rest[1 : len(rest)-1])
		if err != nil {
			return Annotation{}, err
		}
		annotation.Args = args
		annotation.Values = values
		rest = ""
	}
	if rest != "" {
		return Annotation{}, fmt.Errorf("annotation %q has unsupported trailing content %q", annotation.Name, rest)
	}
	return annotation, nil
}

func parseAnnotationArgs(raw string) (map[string]AnnotationArg, []AnnotationArg, error) {
	args := map[string]AnnotationArg{}
	values := make([]AnnotationArg, 0)
	valueFromNamed := false
	if strings.TrimSpace(raw) == "" {
		return args, values, nil
	}
	for _, part := range splitAnnotationArgs(raw) {
		key := "value"
		value := strings.TrimSpace(part)
		if value == "" {
			return nil, nil, fmt.Errorf("annotation argument is empty")
		}
		named := false
		if left, right, ok := cutAnnotationArg(value); ok {
			key = strings.TrimSpace(left)
			value = strings.TrimSpace(right)
			named = true
		}
		if key == "" {
			return nil, nil, fmt.Errorf("annotation argument key is empty")
		}
		if strings.HasPrefix(value, "\"") {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return nil, nil, err
			}
			value = unquoted
		}
		arg := AnnotationArg{text: value}
		if !named {
			if valueFromNamed {
				return nil, nil, fmt.Errorf("duplicate annotation argument %q", "value")
			}
			if _, exists := args["value"]; !exists {
				args["value"] = arg
			}
			values = append(values, arg)
			continue
		}
		if _, exists := args[key]; exists {
			return nil, nil, fmt.Errorf("duplicate annotation argument %q", key)
		}
		args[key] = arg
		if key == "value" {
			valueFromNamed = true
		}
	}
	return args, values, nil
}

func cutAnnotationArg(raw string) (string, string, bool) {
	inQuote := false
	escaped := false
	for index, r := range raw {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == '=' && !inQuote {
			return raw[:index], raw[index+1:], true
		}
	}
	return "", "", false
}

func splitAnnotationArgs(raw string) []string {
	parts := make([]string, 0)
	var builder strings.Builder
	inQuote := false
	escaped := false
	for _, r := range raw {
		if escaped {
			builder.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			builder.WriteRune(r)
			escaped = true
			continue
		}
		if r == '"' {
			builder.WriteRune(r)
			inQuote = !inQuote
			continue
		}
		if r == ',' && !inQuote {
			parts = append(parts, builder.String())
			builder.Reset()
			continue
		}
		builder.WriteRune(r)
	}
	parts = append(parts, builder.String())
	return parts
}

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
				return value.text
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
			values = append(values, value.text)
		}
		return values
	}
	if value, ok := annotation.Args["value"]; ok {
		return []string{value.text}
	}
	return nil
}

func annotationInt(annotations []Annotation, name string, fallback int) int {
	for _, annotation := range annotations {
		if annotation.Name != name {
			continue
		}
		if value, ok := annotation.Args["value"]; ok {
			if parsed, err := strconv.Atoi(value.text); err == nil {
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
	parsed, err := strconv.ParseBool(value.text)
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
		return value.text
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
