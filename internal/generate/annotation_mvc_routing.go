package generate

import (
	"fmt"
	"go/ast"
	"net/http"
	"strconv"
	"strings"

	"goark.dev/cli/internal/generate/annotationmeta"
	"goark.dev/cli/internal/generate/mvcrouting"
)

func mvcHTTPMethods(annotation Annotation) ([]string, bool, error) {
	switch annotation.Name {
	case "get":
		return []string{http.MethodGet}, true, nil
	case "head":
		return []string{http.MethodHead}, true, nil
	case "post":
		return []string{http.MethodPost}, true, nil
	case "put":
		return []string{http.MethodPut}, true, nil
	case "patch":
		return []string{http.MethodPatch}, true, nil
	case "delete":
		return []string{http.MethodDelete}, true, nil
	case "options":
		return []string{http.MethodOptions}, true, nil
	case "trace":
		return []string{http.MethodTrace}, true, nil
	case "request-mapping":
		methods, err := mvcTypeRequestMethodsFromAnnotation(annotation)
		if err != nil {
			return nil, false, err
		}
		if len(methods) == 0 {
			return append([]string(nil), defaultMVCRequestMappingMethods[:]...), false, nil
		}
		return methods, true, nil
	default:
		return nil, false, annotationError("requires supported http method", annotation.Name)
	}
}

func parseMVCRequestMethods(annotation Annotation, value string) ([]string, error) {
	parts := strings.Split(value, ",")
	methods := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		method := strings.ToUpper(strings.TrimSpace(part))
		if method == "" {
			return nil, annotationError("requires supported http method", annotation.Name)
		}
		if !mvcrouting.SupportedMethod(method) {
			return nil, annotationError("requires supported http method", annotation.Name)
		}
		if _, exists := seen[method]; exists {
			continue
		}
		seen[method] = struct{}{}
		methods = append(methods, method)
	}
	if len(methods) == 0 {
		return nil, annotationError("requires supported http method", annotation.Name)
	}
	return methods, nil
}

func mvcStatus(annotation Annotation, fallback int) (int, error) {
	value := firstNonEmpty(
		argString(annotation, "status", ""),
		argString(annotation, "statusCode", ""),
	)
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return parseMVCStatus(annotation, "status", value)
}

func mvcResponseStatus(annotation Annotation) (int, error) {
	values := annotationValueTexts(annotation)
	if len(values) > 1 {
		return 0, annotationError("accepts exactly one status value", annotation.Name)
	}
	value := ""
	if len(values) == 1 {
		value = values[0]
	}
	namedValues := mvcNamedStatusValues(annotation, "status", "statusCode", "code")
	if len(namedValues) > 1 {
		return 0, annotationError("accepts exactly one status argument", annotation.Name)
	}
	if len(namedValues) == 1 {
		if strings.TrimSpace(value) != "" {
			return 0, annotationError(
				"accepts either value or named status argument",
				annotation.Name,
			)
		}
		value = namedValues[0]
	}
	if strings.TrimSpace(value) == "" {
		return 0, annotationError("requires status value", annotation.Name)
	}
	return parseMVCStatus(annotation, "status", value)
}

func mvcNamedStatusValues(annotation Annotation, keys ...string) []string {
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func mvcMappingHasExplicitStatus(annotation Annotation) bool {
	for _, key := range []string{"status", "statusCode"} {
		if strings.TrimSpace(argString(annotation, key, "")) != "" {
			return true
		}
	}
	return false
}

func parseMVCStatus(annotation Annotation, label string, value string) (int, error) {
	status, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, annotationError("%s requires integer value: %w", annotation.Name, label, err)
	}
	if status < 100 || status > 999 {
		return 0, annotationError("%s %d is out of range", annotation.Name, label, status)
	}
	return status, nil
}

func expandMVCRoutePaths(controller *mvcController, route mvcRoute) ([]mvcRoute, error) {
	basePaths := controller.BasePaths
	if len(basePaths) == 0 {
		basePaths = []string{""}
	}
	paths := route.Paths
	if len(paths) == 0 {
		paths = []string{route.Path}
	}
	methods := route.HTTPMethods
	if len(methods) == 0 {
		methods = []string{route.HTTPMethod}
	}
	methods = mvcrouting.CombineMethods(controller.Methods, methods, route.HTTPMethodsSet)
	if len(methods) == 0 {
		return nil, fmt.Errorf(
			"mvc route method %s.%s has no HTTP method after controller request-mapping combination",
			route.ControllerType,
			route.MethodName,
		)
	}
	out := make([]mvcRoute, 0, len(methods)*len(basePaths)*len(paths))
	seen := make(map[string]struct{}, len(methods)*len(basePaths)*len(paths))
	for _, method := range methods {
		for _, basePath := range basePaths {
			for _, path := range paths {
				next := route
				next.HTTPMethod = method
				next.HTTPMethods = nil
				next.Path = mvcrouting.JoinPaths(basePath, path)
				next.Paths = nil
				next.ControllerKind = controller.Kind
				key := next.HTTPMethod + "\x00" + next.Path
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, next)
			}
		}
	}
	return out, nil
}

func isArkWebContextExpr(file *ast.File, expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	return isImportedSelectorExpr(file, star.X, arkartaWebImportPath, "Context")
}

func isGoarkMVCModelPointerExpr(file *ast.File, expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	return isImportedSelectorExpr(file, star.X, goarkMVCImportPath, "Model")
}

func isSelectorTypeExpr(expr ast.Expr, selectorName string) bool {
	star, ok := expr.(*ast.StarExpr)
	if ok {
		expr = star.X
	}
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == selectorName
}

func isErrorExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "error"
}

func isArkWebResultExpr(file *ast.File, expr ast.Expr) bool {
	return isImportedSelectorExpr(file, expr, arkartaWebImportPath, "Result")
}

func isGoarkWebResponseEntityExpr(file *ast.File, expr ast.Expr) bool {
	switch typ := expr.(type) {
	case *ast.IndexExpr:
		return isImportedSelectorExpr(file, typ.X, goarkWebImportPath, "ResponseEntity")
	case *ast.IndexListExpr:
		return isImportedSelectorExpr(file, typ.X, goarkWebImportPath, "ResponseEntity")
	default:
		return false
	}
}

func isGoarkWebDownloadResultExpr(file *ast.File, expr ast.Expr) bool {
	return isImportedSelectorExpr(file, expr, goarkWebImportPath, "DownloadResult")
}

func isImportedSelectorExpr(
	file *ast.File,
	expr ast.Expr,
	importPath string,
	selectorName string,
) bool {
	return annotationmeta.ImportedSelector(file, expr, importPath, selectorName)
}

func importAliases(file *ast.File, importPath string) map[string]struct{} {
	return annotationmeta.ImportAliases(file, importPath)
}
