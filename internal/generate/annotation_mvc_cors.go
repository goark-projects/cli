package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"strconv"
	"strings"
	"time"
)

func ensureMVCAnnotationModel(ctx *AnnotationBindingContext) *mvcAnnotationModel {
	if value, ok := ctx.Value(mvcAnnotationModelKey); ok {
		if model, ok := value.(*mvcAnnotationModel); ok {
			return model
		}
	}
	model := &mvcAnnotationModel{
		byType:       make(map[string]*mvcController),
		adviceByType: make(map[string]*mvcControllerAdvice),
	}
	ctx.SetValue(mvcAnnotationModelKey, model)
	return model
}

func (mvcAnnotationGenerator) GenerateAnnotation(ctx *AnnotationGenerationContext) error {
	value, ok := ctx.Value(mvcAnnotationModelKey)
	if !ok {
		return nil
	}
	model, ok := value.(*mvcAnnotationModel)
	if !ok {
		return fmt.Errorf("invalid mvc annotation model")
	}
	if len(model.Controllers) == 0 && len(model.Advices) == 0 {
		return nil
	}
	ctx.AddImport("", "context")
	ctx.AddImport("", "goark.dev/goark")
	ctx.AddImport("", "goark.dev/goark/container")
	if mvcModelUsesArkWeb(model) {
		ctx.AddImport("arkweb", arkartaWebImportPath)
	}
	if mvcModelUsesConfigurer(model) {
		ctx.AddImport("goweb", "goark.dev/goark/web")
		ctx.AddImport("", "goark.dev/goark/web/mvc")
	}
	if mvcModelUsesCORS(model) {
		ctx.AddImport("", goarkWebCORSImportPath)
	}
	if mvcModelUsesCORSMaxAge(model) {
		ctx.AddImport("", "time")
	}
	if mvcModelUsesOptionalInjection(model) {
		ctx.AddImport("arkerrors", "goark.dev/goark/errors")
	}
	writeMVCConfiguration(ctx.buffer(), model)
	return nil
}

type mvcRouteConditions struct {
	Consumes []string
	Produces []string
	Params   []string
	Headers  []string
}

func mvcRouteConditionsFromAnnotation(annotation Annotation) mvcRouteConditions {
	return mvcRouteConditions{
		Consumes: mvcRouteConditionValues(annotation, "consumes"),
		Produces: mvcRouteConditionValues(annotation, "produces"),
		Params:   mvcRouteConditionValues(annotation, "params"),
		Headers:  mvcRouteConditionValues(annotation, "headers"),
	}
}

func mvcTypeRouteConditions(annotations []Annotation) mvcRouteConditions {
	for _, annotation := range annotations {
		if annotation.Name == "request-mapping" {
			return mvcRouteConditionsFromAnnotation(annotation)
		}
	}
	return mvcRouteConditions{}
}

func mvcRouteConditionValues(annotation Annotation, key string) []string {
	value := strings.TrimSpace(argString(annotation, key, ""))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func writeMVCRouteOptions(builder *bytes.Buffer, conditions mvcRouteConditions) {
	writeMVCMappingOption(builder, ", mvc.With", "Consumes", conditions.Consumes)
	writeMVCMappingOption(builder, ", mvc.With", "Produces", conditions.Produces)
	writeMVCMappingOption(builder, ", mvc.With", "Params", conditions.Params)
	writeMVCMappingOption(builder, ", mvc.With", "Headers", conditions.Headers)
}

func writeMVCControllerOptions(builder *bytes.Buffer, conditions mvcRouteConditions) {
	writeMVCMappingOption(builder, ".With", "Consumes", conditions.Consumes)
	writeMVCMappingOption(builder, ".With", "Produces", conditions.Produces)
	writeMVCMappingOption(builder, ".With", "Params", conditions.Params)
	writeMVCMappingOption(builder, ".With", "Headers", conditions.Headers)
}

func writeMVCMappingOption(builder *bytes.Buffer, prefix string, name string, values []string) {
	if len(values) == 0 {
		return
	}
	builder.WriteString(prefix)
	builder.WriteString(name)
	builder.WriteByte('(')
	for index, value := range values {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(strconv.Quote(value))
	}
	builder.WriteByte(')')
}

func isArkartaMultipartPartExpr(file *ast.File, expr ast.Expr) bool {
	return isImportedSelectorExpr(file, expr, arkartaMultipartImportPath, "Part")
}

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
		return annotationError("does not accept selector", ctx.Annotation.Name)
	}
	switch ctx.Target {
	case AnnotationTargetType:
		if !hasMVCControllerAnnotation(ctx.Item.Annotations()) {
			return annotationError("on type requires mvc controller target", ctx.Annotation.Name)
		}
	case AnnotationTargetMethod:
		if err := validateMVCHandlerMethod(ctx); err != nil {
			return err
		}
		if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
			return annotationError("requires mvc route method target", ctx.Annotation.Name)
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
		return false, annotationError("argument %q requires boolean value: %w", annotation.Name, "allowCredentials", err)
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
		return 0, false, annotationError("argument %q requires duration or seconds: %w", annotation.Name, "maxAge", err)
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
		return "", false, annotationError("accepts exactly one %s argument", annotation.Name, label)
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
