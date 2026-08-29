package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

func validateMVCRequestEntityAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return fmt.Errorf("annotation %q requires mvc route method target", ctx.Annotation.Name)
	}
	selector := mvcRequestEntitySelector(ctx.Annotation)
	if selector == "" {
		return fmt.Errorf("annotation %q requires parameter selector", ctx.Annotation.Name)
	}
	if !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return fmt.Errorf("annotation %q selector %q does not match any method parameter", ctx.Annotation.Name, selector)
	}
	return nil
}

func writeMVCBindRequestEntityHandler(builder *bytes.Buffer, route mvcRoute) {
	bodyParam, _ := mvcRequestEntityParam(route.Handler.Params)
	if len(route.ValidationGroups) > 0 {
		builder.WriteString("mvc.BindRequestEntityGroups[")
	} else {
		builder.WriteString("mvc.BindRequestEntity[")
	}
	builder.WriteString(bodyParam.BodyType)
	builder.WriteString(", any](")
	builder.WriteString(strconv.Itoa(route.Status))
	builder.WriteString(", func(ctx *arkweb.Context, ")
	builder.WriteString(bodyParam.Name)
	builder.WriteByte(' ')
	builder.WriteString(bodyParam.Type)
	builder.WriteString(") (any, error) {\n")
	writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err", route.ValidationGroups)
	builder.WriteString("return ")
	builder.WriteString(mvcHandlerCall(route.MethodName, route.Handler.Params))
	if route.Handler.ReturnKind == mvcReturnValue {
		builder.WriteString(", nil")
	}
	builder.WriteString("\n}")
	writeMVCValidationGroupArguments(builder, route.ValidationGroups)
	builder.WriteByte(')')
}

func writeMVCBindRequestEntityEntityHandler(builder *bytes.Buffer, route mvcRoute) {
	bodyParam, _ := mvcRequestEntityParam(route.Handler.Params)
	if len(route.ValidationGroups) > 0 && route.Handler.EntityBody != "" {
		writeMVCBindRequestEntityEntityGroupsHandler(builder, route)
		return
	}
	builder.WriteString("mvc.BindRequestEntityEntity[")
	builder.WriteString(bodyParam.BodyType)
	builder.WriteString(", ")
	builder.WriteString(route.Handler.EntityBody)
	builder.WriteString("](func(ctx *arkweb.Context, ")
	builder.WriteString(bodyParam.Name)
	builder.WriteByte(' ')
	builder.WriteString(bodyParam.Type)
	builder.WriteString(") (goweb.ResponseEntity[")
	builder.WriteString(route.Handler.EntityBody)
	builder.WriteString("], error) {\n")
	writeMVCParameterBindings(builder, route.Handler.Params, "return goweb.ResponseEntity["+route.Handler.EntityBody+"]{}, err", route.ValidationGroups)
	builder.WriteString("return ")
	builder.WriteString(mvcHandlerCall(route.MethodName, route.Handler.Params))
	if route.Handler.ReturnKind == mvcReturnEntity {
		builder.WriteString(", nil")
	}
	builder.WriteString("\n})")
}

func writeMVCBindRequestEntityEntityGroupsHandler(builder *bytes.Buffer, route mvcRoute) {
	bodyParam, _ := mvcRequestEntityParam(route.Handler.Params)
	builder.WriteString("mvc.BindRequestEntityEntityGroups[")
	builder.WriteString(bodyParam.BodyType)
	builder.WriteString(", ")
	builder.WriteString(route.Handler.EntityBody)
	builder.WriteString("](func(ctx *arkweb.Context, ")
	builder.WriteString(bodyParam.Name)
	builder.WriteByte(' ')
	builder.WriteString(bodyParam.Type)
	builder.WriteString(") (goweb.ResponseEntity[")
	builder.WriteString(route.Handler.EntityBody)
	builder.WriteString("], error) {\n")
	writeMVCParameterBindings(builder, route.Handler.Params, "return goweb.ResponseEntity["+route.Handler.EntityBody+"]{}, err", route.ValidationGroups)
	builder.WriteString("return ")
	builder.WriteString(mvcHandlerCall(route.MethodName, route.Handler.Params))
	if route.Handler.ReturnKind == mvcReturnEntity {
		builder.WriteString(", nil")
	}
	builder.WriteString("\n}")
	writeMVCValidationGroupArguments(builder, route.ValidationGroups)
	builder.WriteByte(')')
}

func isMVCRequestEntityAnnotation(name string) bool {
	return name == "request-entity"
}

func mvcRequestEntitySelectorSet(annotations []Annotation) map[string]struct{} {
	selectors := mvcRequestEntitySelectors(annotations)
	out := make(map[string]struct{}, len(selectors))
	for _, selector := range selectors {
		out[selector] = struct{}{}
	}
	return out
}

func mvcRequestEntitySelectors(annotations []Annotation) []string {
	selectors := make([]string, 0, 1)
	for _, annotation := range annotations {
		if !isMVCRequestEntityAnnotation(annotation.Name) {
			continue
		}
		if selector := mvcRequestEntitySelector(annotation); selector != "" {
			selectors = append(selectors, selector)
		}
	}
	return selectors
}

func mvcRequestEntitySelector(annotation Annotation) string {
	selector := normalizeSelector(annotation.Selector)
	if selector != "" {
		return selector
	}
	for _, key := range []string{"param", "name", "value"} {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			return value
		}
	}
	return ""
}

func hasMVCRequestEntityParam(params []mvcHandlerParam) bool {
	_, ok := mvcRequestEntityParam(params)
	return ok
}

func mvcRequestEntityParam(params []mvcHandlerParam) (mvcHandlerParam, bool) {
	for _, param := range params {
		if param.Kind == mvcParamRequestEntity {
			return param, true
		}
	}
	return mvcHandlerParam{}, false
}

func mvcRequestEntityBodyType(fset *token.FileSet, file *ast.File, expr ast.Expr) (string, bool) {
	switch typ := expr.(type) {
	case *ast.IndexExpr:
		if !isImportedSelectorExpr(file, typ.X, goarkWebImportPath, "RequestEntity") {
			return "", false
		}
		return exprString(fset, typ.Index), true
	case *ast.IndexListExpr:
		if !isImportedSelectorExpr(file, typ.X, goarkWebImportPath, "RequestEntity") || len(typ.Indices) != 1 {
			return "", false
		}
		return exprString(fset, typ.Indices[0]), true
	default:
		return "", false
	}
}
