package generate

import (
	"fmt"
	"go/ast"
	"strings"
)

type mvcParameterBindingItem struct {
	Kind    mvcHandlerParamKind
	Binding mvcParamBinding
}

func mvcParameterBindingSet(annotations []Annotation) (map[string]mvcParameterBindingItem, error) {
	out := make(map[string]mvcParameterBindingItem)
	for _, annotation := range annotations {
		kind, ok := mvcParameterKind(annotation.Name)
		if !ok {
			continue
		}
		selector := mvcBindingSelector(annotation)
		if selector == "" {
			continue
		}
		if _, exists := out[selector]; exists {
			return nil, fmt.Errorf("mvc parameter %q has multiple binding annotations", selector)
		}
		out[selector] = mvcParameterBindingItem{
			Kind: kind,
			Binding: mvcParamBinding{
				SourceName:     mvcParameterSourceName(annotation, selector),
				SourceExplicit: mvcParameterHasSourceName(annotation),
				Required:       mvcParameterRequired(annotation),
				HasDefault:     mvcParameterHasDefault(annotation),
				DefaultValue:   mvcParameterDefaultValue(annotation),
			},
		}
	}
	return out, nil
}

func mvcParameterKind(name string) (mvcHandlerParamKind, bool) {
	switch name {
	case "path-variable":
		return mvcParamPathVariable, true
	case "request-param":
		return mvcParamRequestParam, true
	case "request-header":
		return mvcParamRequestHeader, true
	case "cookie-value":
		return mvcParamCookieValue, true
	case "model-attribute":
		return mvcParamModelAttribute, true
	case "request-attribute":
		return mvcParamRequestAttribute, true
	case "session-attribute":
		return mvcParamSessionAttribute, true
	case "matrix-variable":
		return mvcParamMatrixVariable, true
	case "request-part":
		return mvcParamRequestPart, true
	default:
		return 0, false
	}
}

func mvcBindingSelector(annotation Annotation) string {
	selector := normalizeSelector(annotation.Selector)
	if selector != "" {
		return selector
	}
	return strings.TrimSpace(argString(annotation, "param", ""))
}

func mvcModelAttributeMethodNameAnnotation(annotations []Annotation) string {
	for _, annotation := range annotations {
		if annotation.Name == "model-attribute" {
			return mvcModelAttributeMethodName(annotation)
		}
	}
	return ""
}

func mvcModelAttributeAnnotationCount(annotations []Annotation) int {
	count := 0
	for _, annotation := range annotations {
		if annotation.Name == "model-attribute" {
			count++
		}
	}
	return count
}

func mvcModelAttributeMethodName(annotation Annotation) string {
	values := annotationValueTexts(annotation)
	if len(values) == 1 {
		if value := strings.TrimSpace(values[0]); value != "" {
			return value
		}
	}
	for _, key := range []string{"name", "value"} {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			return value
		}
	}
	return ""
}

func mvcParameterSourceName(annotation Annotation, fallback string) string {
	values := annotationValueTexts(annotation)
	if len(values) == 1 {
		if value := strings.TrimSpace(values[0]); value != "" {
			return value
		}
	}
	for _, key := range []string{"name", "value"} {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			return value
		}
	}
	return fallback
}

func mvcParameterHasSourceName(annotation Annotation) bool {
	if len(annotationValueTexts(annotation)) > 0 {
		return true
	}
	for _, key := range []string{"name", "value"} {
		if strings.TrimSpace(argString(annotation, key, "")) != "" {
			return true
		}
	}
	return false
}

func mvcParameterRequired(annotation Annotation) bool {
	if mvcParameterHasDefault(annotation) {
		return false
	}
	return annotationBool(annotation, "required", true)
}

func mvcParameterHasDefault(annotation Annotation) bool {
	_, ok := annotation.Args["defaultValue"]
	if ok {
		return true
	}
	_, ok = annotation.Args["default"]
	return ok
}

func mvcParameterDefaultValue(annotation Annotation) string {
	if value, ok := annotation.Args["defaultValue"]; ok {
		return value.Text()
	}
	if value, ok := annotation.Args["default"]; ok {
		return value.Text()
	}
	return ""
}

func mvcHasParam(params []mvcHandlerParam, name string) bool {
	for _, param := range params {
		if param.Name == name {
			return true
		}
	}
	return false
}

func hasMVCBodyParam(params []mvcHandlerParam) bool {
	_, ok := mvcBodyParam(params)
	return ok
}

func hasMVCMultipartBodyParam(params []mvcHandlerParam) bool {
	_, ok := mvcMultipartBodyParam(params)
	return ok
}

func hasMVCModelAttributeParam(params []mvcHandlerParam) bool {
	for _, param := range params {
		if param.Kind == mvcParamModelAttribute {
			return true
		}
	}
	return false
}

func hasMVCModelParam(params []mvcHandlerParam) bool {
	_, ok := mvcModelParam(params)
	return ok
}

func mvcBodyParam(params []mvcHandlerParam) (mvcHandlerParam, bool) {
	for _, param := range params {
		if param.Kind == mvcParamBody {
			return param, true
		}
	}
	return mvcHandlerParam{}, false
}

func mvcMultipartBodyParam(params []mvcHandlerParam) (mvcHandlerParam, bool) {
	for _, param := range params {
		if param.Kind == mvcParamMultipartBody {
			return param, true
		}
	}
	return mvcHandlerParam{}, false
}

func mvcModelParam(params []mvcHandlerParam) (mvcHandlerParam, bool) {
	for _, param := range params {
		if param.Kind == mvcParamModel {
			return param, true
		}
	}
	return mvcHandlerParam{}, false
}

func isMVCModelAttributeTypeExpr(expr ast.Expr) bool {
	switch typ := expr.(type) {
	case *ast.StarExpr, *ast.ArrayType, *ast.MapType, *ast.InterfaceType, *ast.FuncType, *ast.ChanType:
		return false
	case *ast.Ident:
		return !isMVCScalarTypeName(typ.Name)
	case *ast.SelectorExpr:
		return true
	default:
		return false
	}
}

func isMVCScalarTypeName(name string) bool {
	switch strings.TrimSpace(name) {
	case "string", "bool",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128",
		"byte", "rune", "any", "error":
		return true
	default:
		return false
	}
}
