package generate

import (
	"bytes"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"goark.dev/cli/internal/generate/mvcrouting"
)

func mvcValidationGroupArguments(groups []string) string {
	if len(groups) == 0 {
		return ""
	}
	var builder bytes.Buffer
	writeMVCValidationGroupArguments(&builder, groups)
	return builder.String()
}

func mvcParameterFunction(kind mvcHandlerParamKind, typ string) (string, bool) {
	suffix, ok := mvcParameterFunctionSuffix(kind, typ)
	if !ok {
		return "", false
	}
	switch kind {
	case mvcParamPathVariable:
		return "Path" + suffix, true
	case mvcParamRequestParam:
		return "RequestParam" + suffix, true
	case mvcParamRequestHeader:
		return "RequestHeader" + suffix, true
	case mvcParamCookieValue:
		return "CookieValue" + suffix, true
	case mvcParamModelAttribute:
		return "ModelAttribute", true
	case mvcParamRequestAttribute:
		return "RequestAttribute" + suffix, true
	case mvcParamSessionAttribute:
		return "SessionAttribute" + suffix, true
	case mvcParamMatrixVariable:
		return "MatrixVariable" + suffix, true
	default:
		return "", false
	}
}

func mvcParameterFunctionSuffix(kind mvcHandlerParamKind, typ string) (string, bool) {
	if kind == mvcParamRequestAttribute || kind == mvcParamSessionAttribute {
		return mvcScalarParameterTypeSuffix(typ)
	}
	return mvcCollectionParameterTypeSuffix(typ)
}

func mvcCollectionParameterTypeSuffix(typ string) (string, bool) {
	switch strings.TrimSpace(typ) {
	case "string":
		return "String", true
	case "int":
		return "Int", true
	case "int64":
		return "Int64", true
	case "bool":
		return "Bool", true
	case "float64":
		return "Float64", true
	case "time.Time":
		return "Time", true
	case "[]string":
		return "Strings", true
	case "[]int":
		return "Ints", true
	case "[]int64":
		return "Int64s", true
	case "[]bool":
		return "Bools", true
	case "[]float64":
		return "Float64s", true
	case "[]time.Time":
		return "Times", true
	default:
		return "", false
	}
}

func mvcScalarParameterTypeSuffix(typ string) (string, bool) {
	switch strings.TrimSpace(typ) {
	case "string":
		return "String", true
	case "int":
		return "Int", true
	case "int64":
		return "Int64", true
	case "bool":
		return "Bool", true
	case "float64":
		return "Float64", true
	case "time.Time":
		return "Time", true
	default:
		return "", false
	}
}

func validateMVCRequestEntityAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return annotationError("requires mvc route method target", ctx.Annotation.Name)
	}
	selector := mvcRequestEntitySelector(ctx.Annotation)
	if selector == "" {
		return annotationError("requires parameter selector", ctx.Annotation.Name)
	}
	if !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return annotationError("selector %q does not match any method parameter", ctx.Annotation.Name, selector)
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

func isMVCRequestEntityAnnotation(name string) bool { return name == "request-entity" }

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

func mvcRequestPartBindingCall(param mvcHandlerParam, validationGroups []string) (string, bool) {
	args := []string{"ctx", strconv.Quote(param.Binding.SourceName)}
	if !param.Binding.Required {
		args = append(args, "mvc.WithRequired(false)")
	}
	if param.RequestPartFile {
		return "mvc.RequestPart(" + strings.Join(args, ", ") + ")", true
	}
	if len(validationGroups) == 0 {
		return "mvc.RequestPartJSON[" + param.Type + "](" + strings.Join(args, ", ") + ")", true
	}
	return "mvc.ValidatedRequestPartJSON[" + param.Type + "](" + strings.Join(mvcValidatedRequestPartArgs(param, validationGroups), ", ") + ")", true
}

func mvcValidatedRequestPartArgs(param mvcHandlerParam, validationGroups []string) []string {
	args := []string{"ctx", strconv.Quote(param.Binding.SourceName)}
	var groups bytes.Buffer
	writeMVCValidationGroupSlice(&groups, validationGroups)
	args = append(args, groups.String())
	if !param.Binding.Required {
		args = append(args, "mvc.WithRequired(false)")
	}
	return args
}

func hasMVCJSONRequestPartParam(params []mvcHandlerParam) bool {
	for _, param := range params {
		if param.Kind == mvcParamRequestPart && !param.RequestPartFile {
			return true
		}
	}
	return false
}

func mvcTypeBasePaths(annotations []Annotation) []string {
	for _, annotation := range annotations {
		if annotation.Name != "request-mapping" {
			continue
		}
		paths, err := requireMVCPathTexts(annotation)
		if err == nil {
			return mvcrouting.NormalizePaths(paths)
		}
	}
	return []string{""}
}

func mvcTypeRequestMethods(annotations []Annotation) ([]string, error) {
	for _, annotation := range annotations {
		if annotation.Name == "request-mapping" {
			return mvcTypeRequestMethodsFromAnnotation(annotation)
		}
	}
	return nil, nil
}

func mvcTypeRequestMethodsFromAnnotation(annotation Annotation) ([]string, error) {
	methods := strings.TrimSpace(argString(annotation, "method", ""))
	if methods == "" {
		return nil, nil
	}
	return parseMVCRequestMethods(annotation, methods)
}

func requireMVCPath(annotation Annotation) error {
	_, err := requireMVCPathTexts(annotation)
	return err
}

func requireMVCPathTexts(annotation Annotation) ([]string, error) {
	values := annotationValueTexts(annotation)
	if len(values) == 0 {
		if value := argString(annotation, "path", ""); value != "" {
			values = []string{value}
		}
	}
	if len(values) == 0 {
		return nil, annotationError("requires path value", annotation.Name)
	}
	paths := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, annotationError("requires path value", annotation.Name)
		}
		paths = append(paths, value)
	}
	return paths, nil
}
