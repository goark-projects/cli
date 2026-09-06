package generate

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func mvcTypeBasePaths(annotations []Annotation) []string {
	for _, annotation := range annotations {
		if annotation.Name != "request-mapping" {
			continue
		}
		paths, err := requireMVCPathTexts(annotation)
		if err == nil {
			return normalizeMVCPaths(paths)
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
		return nil, fmt.Errorf("annotation %q requires path value", annotation.Name)
	}
	paths := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("annotation %q requires path value", annotation.Name)
		}
		paths = append(paths, value)
	}
	return paths, nil
}

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
		return nil, false, fmt.Errorf("annotation %q requires supported http method", annotation.Name)
	}
}

func parseMVCRequestMethods(annotation Annotation, value string) ([]string, error) {
	parts := strings.Split(value, ",")
	methods := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		method := strings.ToUpper(strings.TrimSpace(part))
		if method == "" {
			return nil, fmt.Errorf("annotation %q requires supported http method", annotation.Name)
		}
		if !isSupportedMVCRequestMethod(method) {
			return nil, fmt.Errorf("annotation %q requires supported http method", annotation.Name)
		}
		if _, exists := seen[method]; exists {
			continue
		}
		seen[method] = struct{}{}
		methods = append(methods, method)
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("annotation %q requires supported http method", annotation.Name)
	}
	return methods, nil
}

func isSupportedMVCRequestMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func mvcStatus(annotation Annotation, fallback int) (int, error) {
	value := firstNonEmpty(argString(annotation, "status", ""), argString(annotation, "statusCode", ""))
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return parseMVCStatus(annotation, "status", value)
}

func mvcResponseStatus(annotation Annotation) (int, error) {
	values := annotationValueTexts(annotation)
	if len(values) > 1 {
		return 0, fmt.Errorf("annotation %q accepts exactly one status value", annotation.Name)
	}
	value := ""
	if len(values) == 1 {
		value = values[0]
	}
	namedValues := mvcNamedStatusValues(annotation, "status", "statusCode", "code")
	if len(namedValues) > 1 {
		return 0, fmt.Errorf("annotation %q accepts exactly one status argument", annotation.Name)
	}
	if len(namedValues) == 1 {
		if strings.TrimSpace(value) != "" {
			return 0, fmt.Errorf("annotation %q accepts either value or named status argument", annotation.Name)
		}
		value = namedValues[0]
	}
	if strings.TrimSpace(value) == "" {
		return 0, fmt.Errorf("annotation %q requires status value", annotation.Name)
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
		return 0, fmt.Errorf("annotation %q %s requires integer value: %w", annotation.Name, label, err)
	}
	if status < 100 || status > 999 {
		return 0, fmt.Errorf("annotation %q %s %d is out of range", annotation.Name, label, status)
	}
	return status, nil
}

func defaultMVCStatus(methods []string) int {
	if len(methods) == 1 && methods[0] == http.MethodPost {
		return http.StatusCreated
	}
	return http.StatusOK
}

func routeConstructor(method string) string {
	switch method {
	case http.MethodGet:
		return "GET"
	case http.MethodHead:
		return "HEAD"
	case http.MethodPost:
		return "POST"
	case http.MethodPut:
		return "PUT"
	case http.MethodPatch:
		return "PATCH"
	case http.MethodDelete:
		return "DELETE"
	case http.MethodOptions:
		return "OPTIONS"
	case http.MethodTrace:
		return "TRACE"
	default:
		return "Handle"
	}
}

func normalizeMVCPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(path, "/")
}

func normalizeMVCPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = normalizeMVCPath(path)
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
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
	methods = combineMVCRequestMethods(controller.Methods, methods, route.HTTPMethodsSet)
	if len(methods) == 0 {
		return nil, fmt.Errorf("mvc route method %s.%s has no HTTP method after controller request-mapping combination", route.ControllerType, route.MethodName)
	}
	out := make([]mvcRoute, 0, len(methods)*len(basePaths)*len(paths))
	seen := make(map[string]struct{}, len(methods)*len(basePaths)*len(paths))
	for _, method := range methods {
		for _, basePath := range basePaths {
			for _, path := range paths {
				next := route
				next.HTTPMethod = method
				next.HTTPMethods = nil
				next.Path = joinMVCPaths(basePath, path)
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

func combineMVCRequestMethods(controllerMethods []string, routeMethods []string, routeMethodsSet bool) []string {
	if len(controllerMethods) == 0 {
		return routeMethods
	}
	if !routeMethodsSet {
		return append([]string(nil), controllerMethods...)
	}
	out := append([]string(nil), routeMethods...)
	seen := make(map[string]struct{}, len(routeMethods)+len(controllerMethods))
	for _, method := range routeMethods {
		seen[method] = struct{}{}
	}
	for _, method := range controllerMethods {
		if _, exists := seen[method]; exists {
			continue
		}
		seen[method] = struct{}{}
		out = append(out, method)
	}
	return out
}

func joinMVCPaths(base string, path string) string {
	base = normalizeMVCPath(base)
	path = normalizeMVCPath(path)
	if base == "/" {
		return path
	}
	if path == "/" {
		return base
	}
	return base + path
}
