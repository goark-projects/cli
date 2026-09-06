package generate

import (
	"strings"
)

func hasMVCControllerAnnotation(annotations []Annotation) bool {
	return mvcControllerKind(annotations) != ""
}

func mvcControllerKind(annotations []Annotation) string {
	for _, name := range []string{"controller", "rest-controller", "mvc-controller"} {
		if hasAnnotation(annotations, name) {
			return name
		}
	}
	return ""
}

func hasMVCRouteMappingAnnotation(annotations []Annotation) bool {
	for _, annotation := range annotations {
		if isMVCRouteMappingAnnotation(annotation.Name) {
			return true
		}
	}
	return false
}

func isMVCRouteAnnotation(name string) bool {
	return isMVCRouteMappingAnnotation(name) || isMVCBodyAnnotation(name) || isMVCRequestEntityAnnotation(name) || isMVCMultipartBodyAnnotation(name) || isMVCParameterAnnotation(name) || isMVCValidatedAnnotation(name) || isMVCResponseBodyAnnotation(name) || isMVCResponseStatusAnnotation(name) || isMVCCrossOriginAnnotation(name)
}

func isMVCRouteMappingAnnotation(name string) bool {
	switch name {
	case "request-mapping", "get", "head", "post", "put", "patch", "delete", "options", "trace":
		return true
	default:
		return false
	}
}

func isMVCBodyAnnotation(name string) bool {
	switch name {
	case "request-body", "body":
		return true
	default:
		return false
	}
}

func isMVCMultipartBodyAnnotation(name string) bool {
	return name == "multipart-body"
}

func isMVCValidatedAnnotation(name string) bool {
	return name == "validated"
}

func isMVCParameterAnnotation(name string) bool {
	switch name {
	case "path-variable", "request-param", "request-header", "cookie-value", "model-attribute",
		"request-attribute", "session-attribute", "matrix-variable", "request-part":
		return true
	default:
		return false
	}
}

func isMVCResponseStatusAnnotation(name string) bool {
	return name == "response-status"
}

func hasMVCResponseBodyAnnotation(annotations []Annotation) bool {
	return hasAnnotation(annotations, "response-body")
}

func hasMVCValidatedAnnotation(annotations []Annotation) bool {
	return hasAnnotation(annotations, "validated")
}

func isMVCResponseBodyAnnotation(name string) bool {
	return name == "response-body"
}

func mvcValidationGroups(annotation Annotation) []string {
	values := annotationValueTexts(annotation)
	for _, key := range []string{"groups", "group"} {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			values = append(values, value)
		}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func mvcRequestBodySelectorSet(annotations []Annotation) map[string]struct{} {
	selectors := mvcRequestBodySelectors(annotations)
	out := make(map[string]struct{}, len(selectors))
	for _, selector := range selectors {
		out[selector] = struct{}{}
	}
	return out
}

func mvcRequestBodySelectors(annotations []Annotation) []string {
	selectors := make([]string, 0, 1)
	for _, annotation := range annotations {
		if !isMVCBodyAnnotation(annotation.Name) {
			continue
		}
		if selector := mvcRequestBodySelector(annotation); selector != "" {
			selectors = append(selectors, selector)
		}
	}
	return selectors
}

func mvcRequestBodySelector(annotation Annotation) string {
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

func mvcMultipartBodySelectorSet(annotations []Annotation) map[string]struct{} {
	selectors := mvcMultipartBodySelectors(annotations)
	out := make(map[string]struct{}, len(selectors))
	for _, selector := range selectors {
		out[selector] = struct{}{}
	}
	return out
}

func mvcMultipartBodySelectors(annotations []Annotation) []string {
	selectors := make([]string, 0, 1)
	for _, annotation := range annotations {
		if !isMVCMultipartBodyAnnotation(annotation.Name) {
			continue
		}
		if selector := mvcMultipartBodySelector(annotation); selector != "" {
			selectors = append(selectors, selector)
		}
	}
	return selectors
}

func mvcMultipartBodySelector(annotation Annotation) string {
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
