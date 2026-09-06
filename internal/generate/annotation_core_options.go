package generate

import (
	"strings"
)

func componentKind(annotations []Annotation) string {
	for _, name := range []string{"component", "service", "repository"} {
		if hasAnnotation(annotations, name) {
			return name
		}
	}
	return ""
}

func componentOptionKind(annotations []Annotation) string {
	if kind := componentKind(annotations); kind != "" {
		return kind
	}
	return webComponentKind(annotations)
}

func buildBeanOptions(annotations []Annotation) annotationBeanOptions {
	options := annotationBeanOptions{
		Primary: hasAnnotation(annotations, "primary"),
		Lazy:    annotationBoolByName(annotations, "lazy", true),
		Scope:   annotationString(annotations, "scope", ""),
	}
	if !hasAnnotation(annotations, "lazy") {
		options.Lazy = false
	}
	if values := annotationStrings(annotations, "depends-on"); len(values) > 0 {
		for _, value := range values {
			for _, item := range strings.Split(value, ",") {
				item = strings.TrimSpace(item)
				if item != "" {
					options.DependsOn = append(options.DependsOn, item)
				}
			}
		}
	}
	if hasAnnotation(annotations, "order") {
		value := annotationInt(annotations, "order", 0)
		options.Order = &value
	}
	if hasAnnotation(annotations, "priority") {
		value := annotationInt(annotations, "priority", 0)
		options.Priority = &value
	}
	return options
}

func propertySourceAnnotations(annotations []Annotation) []annotationPropertySource {
	sources := make([]annotationPropertySource, 0)
	for _, annotation := range annotations {
		switch annotation.Name {
		case "property-source":
			source := annotationPropertySource{
				Location:               argString(annotation, "value", ""),
				Name:                   argString(annotation, "name", ""),
				Encoding:               argString(annotation, "encoding", ""),
				IgnoreResourceNotFound: annotationBool(annotation, "ignoreResourceNotFound", false),
			}
			if source.Location != "" {
				sources = append(sources, source)
			}
		case "property-sources":
			for _, location := range strings.Split(argString(annotation, "value", ""), ";") {
				location = strings.TrimSpace(location)
				if location != "" {
					sources = append(sources, annotationPropertySource{Location: location})
				}
			}
		}
	}
	return sources
}

func buildInjection(annotations []Annotation, defaultName string) injectionSpec {
	injection := injectionSpec{Required: true}
	if value := annotationString(annotations, "value", ""); value != "" {
		injection.Kind = "value"
		injection.Value = value
		return injection
	}
	qualifier := firstNonEmpty(
		annotationString(annotations, "qualifier", ""),
		annotationString(annotations, "named", ""),
		autowiredQualifier(annotations),
	)
	if hasAnnotation(annotations, "resource") {
		injection.Kind = "resource"
		injection.Qualifier = annotationString(annotations, "resource", defaultName)
		return injection
	}
	if hasAnnotation(annotations, "inject") || hasAnnotation(annotations, "autowired") || qualifier != "" {
		injection.Kind = "bean"
		injection.Qualifier = qualifier
		if hasAnnotation(annotations, "autowired") {
			injection.Required = autowiredRequired(annotations)
		}
	}
	return injection
}

func autowiredQualifier(annotations []Annotation) string {
	for _, annotation := range annotations {
		if annotation.Name != "autowired" {
			continue
		}
		if value, ok := annotation.Args["qualifier"]; ok {
			return value.Text()
		}
	}
	return ""
}

func autowiredRequired(annotations []Annotation) bool {
	for _, annotation := range annotations {
		if annotation.Name != "autowired" {
			continue
		}
		return annotationBool(annotation, "required", true)
	}
	return true
}
