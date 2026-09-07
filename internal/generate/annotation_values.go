package generate

import (
	"bytes"
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"goark.dev/cli/internal/generate/annotationmeta"
	"goark.dev/cli/internal/generate/annotationparse"
)

func addConfigurationPropertiesImports(
	ctx *AnnotationGenerationContext,
	properties []annotationConfigurationProperties,
) {
	for _, item := range properties {
		for _, importSpec := range item.Imports {
			ctx.AddImport(importSpec.Alias, importSpec.Path)
		}
	}
}

func writeConfigurationProperties(
	builder *bytes.Buffer,
	properties annotationConfigurationProperties,
) {
	sourceType := properties.TypeName
	if properties.SourceTypeName != "" {
		sourceType = properties.SourceTypeName
	}
	builder.WriteString("// Bind")
	builder.WriteString(properties.TypeName)
	builder.WriteString(" 从 Environment 绑定配置属性。\nfunc Bind")
	builder.WriteString(properties.TypeName)
	builder.WriteString("(environment goark.Environment) (out *")
	builder.WriteString(sourceType)
	builder.WriteString(", err error) {\nout = &")
	builder.WriteString(sourceType)
	builder.WriteString("{}\n")
	for _, initializer := range properties.Initializers {
		builder.WriteString(initializer)
		builder.WriteByte('\n')
	}
	for _, field := range properties.Fields {
		writeConfigurationPropertyBinding(builder, field)
	}
	if !properties.IgnoreUnknownFields {
		builder.WriteString("if err = goark.ValidateConfigurationPropertyNames(environment, ")
		builder.WriteString(strconv.Quote(properties.Prefix))
		builder.WriteString(", []string{")
		for _, field := range properties.Fields {
			name := field.Name
			if field.MapValueType != "" {
				name += ".*"
			}
			builder.WriteString(strconv.Quote(name))
			builder.WriteByte(',')
		}
		builder.WriteString("}); err != nil {\nreturn nil, err\n}\n")
	}
	writeConfigurationValidator(builder)
	builder.WriteString("return out, nil\n}\n\n")
	writeConfigurationPropertiesMetadata(builder, properties)
}

func writeConfigurationPropertyBinding(
	builder *bytes.Buffer,
	field annotationConfigurationPropertyField,
) {
	if field.MapValueType != "" {
		writeConfigurationPropertyMapBinding(builder, field)
		return
	}
	builder.WriteString("if value, found, bindErr := coreenv.GetPropertyAsValue[")
	builder.WriteString(field.Type)
	builder.WriteString("](environment, ")
	builder.WriteString(strconv.Quote(field.Name))
	builder.WriteString(
		"); bindErr != nil {\nreturn nil, " +
			"arkerrors.Wrapf(arkerrors.CodeConversion, bindErr, " +
			"\"failed to bind configuration property %s\", ",
	)
	builder.WriteString(strconv.Quote(field.Name))
	builder.WriteString(")\n} else if found {\n")
	builder.WriteString(field.Target)
	builder.WriteString(" = value\n}")
	if field.DefaultValue != "" {
		builder.WriteString(" else {\n")
		builder.WriteString(field.Target)
		builder.WriteString(", err = coreenv.ResolveValueAs[")
		builder.WriteString(field.Type)
		builder.WriteString("](environment, ")
		builder.WriteString(strconv.Quote(field.DefaultValue))
		builder.WriteString(")\nif err != nil {\nreturn nil, err\n}\n}")
	} else if field.Required {
		builder.WriteString(" else {\nreturn nil, " +
			"arkerrors.Newf(arkerrors.CodeNotFound, " +
			"\"required configuration property %q not found\", ")
		builder.WriteString(strconv.Quote(field.Name))
		builder.WriteString(")\n}")
	}
	builder.WriteByte('\n')
}

func writeConfigurationPropertyMapBinding(
	builder *bytes.Buffer,
	field annotationConfigurationPropertyField,
) {
	builder.WriteString("if value, found, bindErr := coreenv.GetPropertyMapAsValue[")
	builder.WriteString(field.MapValueType)
	builder.WriteString("](environment, ")
	builder.WriteString(strconv.Quote(field.Name))
	builder.WriteString("); bindErr != nil {\nreturn nil, bindErr\n} else if found {\n")
	builder.WriteString(field.Target)
	builder.WriteString(" = value\n}\n")
}

func writeConfigurationPropertiesMetadata(
	builder *bytes.Buffer,
	properties annotationConfigurationProperties,
) {
	builder.WriteString("// ")
	builder.WriteString(properties.TypeName)
	builder.WriteString("ConfigurationMetadata 返回生成的配置属性元数据。\nfunc ")
	builder.WriteString(properties.TypeName)
	builder.WriteString(
		"ConfigurationMetadata() []goark.ConfigurationProperty {\n" +
			"return []goark.ConfigurationProperty{\n",
	)
	for _, field := range properties.Fields {
		builder.WriteString("{Name:")
		builder.WriteString(strconv.Quote(field.Name))
		builder.WriteString(",Type:")
		builder.WriteString(strconv.Quote(field.Type))
		builder.WriteString(",DefaultValue:")
		builder.WriteString(strconv.Quote(field.DefaultValue))
		builder.WriteString(",Required:")
		builder.WriteString(strconv.FormatBool(field.Required))
		builder.WriteString("},\n")
	}
	builder.WriteString("}\n}\n\n")
}

func writeConfigurationPropertiesRegistration(
	builder *bytes.Buffer,
	properties annotationConfigurationProperties,
) {
	sourceType := properties.TypeName
	if properties.SourceTypeName != "" {
		sourceType = properties.SourceTypeName
	}
	builder.WriteString("if err := container.Register(registry, ")
	builder.WriteString(strconv.Quote(properties.BeanName))
	builder.WriteString(", func(_ context.Context, _ container.Resolver) (*")
	builder.WriteString(sourceType)
	builder.WriteString(", error) {\nreturn Bind")
	builder.WriteString(properties.TypeName)
	builder.WriteString("(config.Environment())\n}); err != nil {\nreturn err\n}\n")
}

// Annotation 表示一条 //goark:* 注解。
type Annotation = annotationparse.Annotation

// AnnotationArg 表示注解参数。
type AnnotationArg = annotationparse.AnnotationArg

func parseAnnotations(group *ast.CommentGroup) ([]Annotation, error) {
	return annotationparse.ParseComments(group)
}

func mergeAnnotations(left []Annotation, right []Annotation) []Annotation {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	out := make([]Annotation, 0, len(left)+len(right))
	out = append(out, left...)
	out = append(out, right...)
	return out
}

func (c *AnnotationGenerationContext) buffer() *bytes.Buffer {
	return &c.body
}

func hasAnnotation(annotations []Annotation, name string) bool {
	return annotationmeta.Has(annotations, name)
}

func annotationHasArguments(annotation Annotation) bool {
	return normalizeSelector(annotation.Selector) != "" ||
		len(annotation.Args) > 0 || len(annotation.Values) > 0
}

func annotationSelectorError(name string, selector string) error {
	return annotationError("selector %q does not match any method parameter", name, selector)
}

func annotationRouteSelectorError(name string, selector string) error {
	return annotationError("selector %q requires mvc route method target", name, selector)
}

func hasMVCMappedHandler(annotations []Annotation) bool {
	return hasMVCRouteMappingAnnotation(annotations) || hasMVCExceptionHandlerAnnotation(annotations)
}

func annotationMappedTargetError(name string) error {
	return annotationError("requires mvc route or exception handler method target", name)
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

func mvcHandlerSupportsValidation(params []mvcHandlerParam) bool {
	return hasMVCBodyParam(params) || hasMVCRequestEntityParam(params) ||
		hasMVCMultipartBodyParam(params) || hasMVCModelAttributeParam(params) ||
		hasMVCJSONRequestPartParam(params)
}

func annotationName(annotations []Annotation, name string, fallback string) string {
	if value := annotationString(annotations, name, ""); value != "" {
		return value
	}
	return fallback
}

func annotationString(annotations []Annotation, name string, fallback string) string {
	return annotationmeta.String(annotations, name, fallback)
}

func annotationStrings(annotations []Annotation, name string) []string {
	return annotationmeta.Strings(annotations, name)
}

func annotationValueTexts(annotation Annotation) []string {
	return annotationmeta.ValueTexts(annotation)
}

func annotationInt(annotations []Annotation, name string, fallback int) int {
	return annotationmeta.Int(annotations, name, fallback)
}

func annotationBool(annotation Annotation, key string, fallback bool) bool {
	return annotationmeta.Bool(annotation, key, fallback)
}

func annotationBoolByName(annotations []Annotation, name string, fallback bool) bool {
	return annotationmeta.BoolByName(annotations, name, fallback)
}

func argString(annotation Annotation, key string, fallback string) string {
	return annotationmeta.ArgString(annotation, key, fallback)
}

func annotationsBySelector(annotations []Annotation) map[string][]Annotation {
	return annotationmeta.BySelector(annotations)
}

func receiverTypeName(recv *ast.FieldList) string {
	return annotationmeta.ReceiverTypeName(recv)
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	return annotationmeta.ExprString(fset, expr)
}

func wrapExpressions(expressions []string) []string {
	return annotationmeta.WrapExpressions(expressions)
}

func firstNonEmpty(values ...string) string {
	return annotationmeta.FirstNonEmpty(values...)
}

func lowerCamel(value string) string {
	return annotationmeta.LowerCamel(value)
}

// ImportSpec 描述生成文件所需的额外导入。
type ImportSpec struct {
	Alias string
	Path  string
}

const (
	ScopeSingleton = "singleton"
	ScopePrototype = "prototype"
)

func sortImports(imports []ImportSpec) {
	sort.SliceStable(imports, func(left int, right int) bool {
		if imports[left].Path == imports[right].Path {
			return imports[left].Alias < imports[right].Alias
		}
		return imports[left].Path < imports[right].Path
	})
}

// AddConfigurationType 记录一个由注解生成器实际输出的配置类型。
func (c *AnnotationBindingContext) AddConfigurationType(name string) {
	value, _ := c.Value(ConfigurationTypesModelKey)
	types, _ := value.([]string)
	c.SetValue(ConfigurationTypesModelKey, append(types, name))
}
