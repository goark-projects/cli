package generate

import (
	"fmt"
	"go/ast"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type annotationConfigurationProperties struct {
	TypeName            string
	BeanName            string
	Prefix              string
	IgnoreUnknownFields bool
	Fields              []annotationConfigurationPropertyField
	Initializers        []string
	Imports             []ImportSpec
}

type annotationConfigurationPropertyField struct {
	Target       string
	Name         string
	Type         string
	DefaultValue string
	Required     bool
}

func validateConfigurationPropertiesAnnotation(ctx AnnotationValidationContext) error {
	if err := validateCoreStructTypeAnnotation(ctx); err != nil {
		return err
	}
	if err := validateAtMostOneAnnotationValue(ctx.Annotation); err != nil {
		return err
	}
	if _, hasPrefix := ctx.Annotation.Args["prefix"]; hasPrefix && len(ctx.Annotation.Values) > 0 {
		return fmt.Errorf("annotation %q accepts either prefix or value argument", ctx.Annotation.Name)
	}
	return validateBoolArg(ctx.Annotation, "ignoreUnknownFields")
}

func buildConfigurationProperties(ctx *AnnotationBindingContext, item AnnotationItem) (annotationConfigurationProperties, error) {
	annotation := findAnnotation(item.annotations, "configuration-properties")
	prefix := strings.Trim(argString(annotation, "prefix", argString(annotation, "value", "")), ".")
	properties := annotationConfigurationProperties{
		TypeName:            item.TypeName(),
		BeanName:            lowerCamel(item.TypeName()),
		Prefix:              prefix,
		IgnoreUnknownFields: annotationBool(annotation, "ignoreUnknownFields", true),
	}
	imports := make(map[string]ImportSpec)
	visiting := make(map[string]bool)
	if err := collectConfigurationPropertyFields(ctx, item.TypeName(), "out", prefix, visiting, imports, &properties); err != nil {
		return annotationConfigurationProperties{}, err
	}
	for _, item := range imports {
		properties.Imports = append(properties.Imports, item)
	}
	sort.Slice(properties.Imports, func(i, j int) bool {
		return properties.Imports[i].Path < properties.Imports[j].Path
	})
	return properties, nil
}

func collectConfigurationPropertyFields(
	ctx *AnnotationBindingContext,
	typeName string,
	target string,
	prefix string,
	visiting map[string]bool,
	imports map[string]ImportSpec,
	properties *annotationConfigurationProperties,
) error {
	if visiting[typeName] {
		return fmt.Errorf("configuration properties type %s contains recursive struct reference", typeName)
	}
	declaration, ok := ctx.pkg.types[typeName]
	if !ok {
		return fmt.Errorf("configuration properties type %s is not declared in package", typeName)
	}
	structure, ok := declaration.spec.Type.(*ast.StructType)
	if !ok {
		return fmt.Errorf("configuration properties type %s must be a struct", typeName)
	}
	visiting[typeName] = true
	defer delete(visiting, typeName)
	for _, field := range structure.Fields.List {
		if len(field.Names) != 1 || !ast.IsExported(field.Names[0].Name) {
			continue
		}
		fieldName := field.Names[0].Name
		tag, err := parseConfigurationPropertyTag(field.Tag)
		if err != nil {
			return fmt.Errorf("configuration properties %s.%s: %w", typeName, fieldName, err)
		}
		if tag.skip {
			continue
		}
		propertySegment := tag.name
		if propertySegment == "" {
			propertySegment = kebabCase(fieldName)
		}
		propertyName := joinPropertyName(prefix, propertySegment)
		fieldTarget := target + "." + fieldName
		if nestedType, pointer := localStructType(ctx.pkg, field.Type); nestedType != "" {
			if pointer {
				properties.Initializers = append(properties.Initializers, fieldTarget+" = &"+nestedType+"{}")
			}
			if err := collectConfigurationPropertyFields(ctx, nestedType, fieldTarget, propertyName, visiting, imports, properties); err != nil {
				return err
			}
			continue
		}
		if _, ok := field.Type.(*ast.MapType); ok {
			return fmt.Errorf("configuration properties field %s.%s uses map type, which is not supported yet", typeName, fieldName)
		}
		collectTypeImports(declaration.file, field.Type, imports)
		properties.Fields = append(properties.Fields, annotationConfigurationPropertyField{
			Target:       fieldTarget,
			Name:         propertyName,
			Type:         exprString(ctx.pkg.fset, field.Type),
			DefaultValue: tag.defaultValue,
			Required:     tag.required,
		})
	}
	return nil
}

type configurationPropertyTag struct {
	name         string
	defaultValue string
	required     bool
	skip         bool
}

func parseConfigurationPropertyTag(tag *ast.BasicLit) (configurationPropertyTag, error) {
	if tag == nil {
		return configurationPropertyTag{}, nil
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return configurationPropertyTag{}, err
	}
	value := reflect.StructTag(raw).Get("goark")
	if value == "" {
		return configurationPropertyTag{}, nil
	}
	parts := strings.Split(value, ",")
	result := configurationPropertyTag{name: strings.TrimSpace(parts[0])}
	if result.name == "-" {
		result.skip = true
		return result, nil
	}
	for _, option := range parts[1:] {
		option = strings.TrimSpace(option)
		switch {
		case option == "required":
			result.required = true
		case strings.HasPrefix(option, "default="):
			result.defaultValue = strings.TrimPrefix(option, "default=")
		case option != "":
			return configurationPropertyTag{}, fmt.Errorf("unsupported goark tag option %q", option)
		}
	}
	if result.required && result.defaultValue != "" {
		return configurationPropertyTag{}, fmt.Errorf("required and default cannot be combined")
	}
	return result, nil
}

func localStructType(pkg *annotationPackage, expression ast.Expr) (string, bool) {
	pointer := false
	if value, ok := expression.(*ast.StarExpr); ok {
		pointer = true
		expression = value.X
	}
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return "", false
	}
	declaration, ok := pkg.types[identifier.Name]
	if !ok {
		return "", false
	}
	if _, ok := declaration.spec.Type.(*ast.StructType); !ok {
		return "", false
	}
	return identifier.Name, pointer
}

func collectTypeImports(file *ast.File, expression ast.Expr, imports map[string]ImportSpec) {
	qualifiers := make(map[string]struct{})
	ast.Inspect(expression, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		identifier, ok := selector.X.(*ast.Ident)
		if ok {
			qualifiers[identifier.Name] = struct{}{}
		}
		return true
	})
	for _, importSpec := range file.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			continue
		}
		alias := path.Base(importPath)
		if importSpec.Name != nil {
			alias = importSpec.Name.Name
		}
		if _, used := qualifiers[alias]; used {
			imports[alias+"\x00"+importPath] = ImportSpec{Alias: importAlias(alias, importPath), Path: importPath}
		}
	}
}

func importAlias(alias string, importPath string) string {
	if alias == path.Base(importPath) {
		return ""
	}
	return alias
}

func kebabCase(value string) string {
	runes := []rune(value)
	var builder strings.Builder
	for index, current := range runes {
		if unicode.IsUpper(current) {
			previousIsLowerOrDigit := index > 0 && (unicode.IsLower(runes[index-1]) || unicode.IsDigit(runes[index-1]))
			nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			if index > 0 && (previousIsLowerOrDigit || nextIsLower) {
				builder.WriteByte('-')
			}
			builder.WriteRune(unicode.ToLower(current))
			continue
		}
		builder.WriteRune(current)
	}
	return builder.String()
}

func joinPropertyName(prefix string, name string) string {
	if prefix == "" {
		return name
	}
	if name == "" {
		return prefix
	}
	return prefix + "." + name
}

func findAnnotation(annotations []Annotation, name string) Annotation {
	for _, annotation := range annotations {
		if annotation.Name == name {
			return annotation
		}
	}
	return Annotation{}
}
