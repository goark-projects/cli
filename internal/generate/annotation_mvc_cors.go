package generate

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type mvcCrossOrigin struct {
	AllowedOrigins        []string
	AllowedOriginPatterns []string
	AllowedMethods        []string
	AllowedHeaders        []string
	ExposedHeaders        []string
	AllowCredentials      bool
	MaxAge                int64
	MaxAgeSet             bool
}

func validateMVCCrossOriginAnnotation(ctx AnnotationValidationContext) error {
	if selector := normalizeSelector(ctx.Annotation.Selector); selector != "" {
		return fmt.Errorf("annotation %q does not accept selector", ctx.Annotation.Name)
	}
	switch ctx.Target {
	case AnnotationTargetType:
		if !hasMVCControllerAnnotation(ctx.Item.Annotations()) {
			return fmt.Errorf("annotation %q on type requires mvc controller target", ctx.Annotation.Name)
		}
	case AnnotationTargetMethod:
		if err := validateMVCHandlerMethod(ctx); err != nil {
			return err
		}
		if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
			return fmt.Errorf("annotation %q requires mvc route method target", ctx.Annotation.Name)
		}
	}
	_, err := mvcCrossOriginFromAnnotation(ctx.Annotation)
	return err
}

func mvcCrossOriginFromAnnotations(annotations []Annotation) (*mvcCrossOrigin, error) {
	var out *mvcCrossOrigin
	for _, annotation := range annotations {
		if !isMVCCrossOriginAnnotation(annotation.Name) {
			continue
		}
		if out != nil {
			return nil, fmt.Errorf("mvc target has multiple cross-origin annotations")
		}
		config, err := mvcCrossOriginFromAnnotation(annotation)
		if err != nil {
			return nil, err
		}
		out = &config
	}
	return out, nil
}

func mvcCrossOriginFromAnnotation(annotation Annotation) (mvcCrossOrigin, error) {
	allowCredentials, err := mvcCrossOriginBool(annotation, "allowCredentials", "allow-credentials", "credentials")
	if err != nil {
		return mvcCrossOrigin{}, err
	}
	maxAge, maxAgeSet, err := mvcCrossOriginMaxAge(annotation)
	if err != nil {
		return mvcCrossOrigin{}, err
	}
	return mvcCrossOrigin{
		AllowedOrigins: mvcCrossOriginList(annotation, true,
			"origins", "origin", "allowedOrigins", "allowed-origins"),
		AllowedOriginPatterns: mvcCrossOriginList(annotation, false,
			"originPatterns", "origin-patterns", "allowedOriginPatterns", "allowed-origin-patterns"),
		AllowedMethods: mvcCrossOriginList(annotation, false,
			"methods", "method", "allowedMethods", "allowed-methods"),
		AllowedHeaders: mvcCrossOriginList(annotation, false,
			"allowedHeaders", "allowed-headers", "headers"),
		ExposedHeaders: mvcCrossOriginList(annotation, false,
			"exposedHeaders", "exposed-headers"),
		AllowCredentials: allowCredentials,
		MaxAge:           maxAge,
		MaxAgeSet:        maxAgeSet,
	}, nil
}

func mvcCrossOriginList(annotation Annotation, includeValues bool, keys ...string) []string {
	values := make([]string, 0, len(keys)+len(annotation.Values))
	if includeValues {
		values = append(values, annotationValueTexts(annotation)...)
	}
	for _, key := range keys {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			values = append(values, value)
		}
	}
	return splitMVCCrossOriginValues(values)
}

func splitMVCCrossOriginValues(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			out = append(out, part)
		}
	}
	return out
}

func mvcCrossOriginBool(annotation Annotation, keys ...string) (bool, error) {
	value, ok, err := mvcCrossOriginSingleArg(annotation, "allowCredentials", keys...)
	if err != nil || !ok {
		return false, err
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("annotation %q argument %q requires boolean value: %w", annotation.Name, "allowCredentials", err)
	}
	return parsed, nil
}

func mvcCrossOriginMaxAge(annotation Annotation) (int64, bool, error) {
	value, ok, err := mvcCrossOriginSingleArg(annotation, "maxAge", "maxAge", "max-age")
	if err != nil || !ok {
		return 0, false, err
	}
	duration, err := parseMVCCrossOriginDuration(value)
	if err != nil {
		return 0, false, fmt.Errorf("annotation %q argument %q requires duration or seconds: %w", annotation.Name, "maxAge", err)
	}
	return int64(duration), true, nil
}

func mvcCrossOriginSingleArg(annotation Annotation, label string, keys ...string) (string, bool, error) {
	values := make([]string, 0, 1)
	for _, key := range keys {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return "", false, nil
	}
	if len(values) > 1 {
		return "", false, fmt.Errorf("annotation %q accepts exactly one %s argument", annotation.Name, label)
	}
	return values[0], true, nil
}

func parseMVCCrossOriginDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if duration, err := time.ParseDuration(value); err == nil {
		return duration, nil
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds) * time.Second, nil
}

func writeMVCCrossOriginRouteOption(builder *bytes.Buffer, config *mvcCrossOrigin) {
	if config == nil {
		return
	}
	builder.WriteString(", mvc.WithCrossOrigin(")
	writeMVCCrossOriginConfig(builder, config)
	builder.WriteByte(')')
}

func writeMVCControllerCrossOrigin(builder *bytes.Buffer, config *mvcCrossOrigin) {
	if config == nil {
		return
	}
	builder.WriteString(".WithCrossOrigin(")
	writeMVCCrossOriginConfig(builder, config)
	builder.WriteByte(')')
}

func writeMVCCrossOriginConfig(builder *bytes.Buffer, config *mvcCrossOrigin) {
	builder.WriteString("cors.Config{")
	writeMVCCrossOriginStringSliceField(builder, "AllowedOrigins", config.AllowedOrigins)
	writeMVCCrossOriginStringSliceField(builder, "AllowedOriginPatterns", config.AllowedOriginPatterns)
	writeMVCCrossOriginStringSliceField(builder, "AllowedMethods", config.AllowedMethods)
	writeMVCCrossOriginStringSliceField(builder, "AllowedHeaders", config.AllowedHeaders)
	writeMVCCrossOriginStringSliceField(builder, "ExposedHeaders", config.ExposedHeaders)
	if config.AllowCredentials {
		builder.WriteString("AllowCredentials: true,")
	}
	if config.MaxAgeSet {
		builder.WriteString("MaxAge: time.Duration(")
		builder.WriteString(strconv.FormatInt(config.MaxAge, 10))
		builder.WriteString("),")
	}
	builder.WriteByte('}')
}

func writeMVCCrossOriginStringSliceField(builder *bytes.Buffer, name string, values []string) {
	if len(values) == 0 {
		return
	}
	builder.WriteString(name)
	builder.WriteString(": []string{")
	for index, value := range values {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(strconv.Quote(value))
	}
	builder.WriteString("},")
}

func isMVCCrossOriginAnnotation(name string) bool {
	return name == "cross-origin"
}

func mvcModelUsesCORS(model *mvcAnnotationModel) bool {
	if model == nil {
		return false
	}
	for _, controller := range model.Controllers {
		if controller.CrossOrigin != nil {
			return true
		}
		for _, route := range controller.Routes {
			if route.CrossOrigin != nil {
				return true
			}
		}
	}
	return false
}

func mvcModelUsesCORSMaxAge(model *mvcAnnotationModel) bool {
	if model == nil {
		return false
	}
	for _, controller := range model.Controllers {
		if controller.CrossOrigin != nil && controller.CrossOrigin.MaxAgeSet {
			return true
		}
		for _, route := range controller.Routes {
			if route.CrossOrigin != nil && route.CrossOrigin.MaxAgeSet {
				return true
			}
		}
	}
	return false
}
