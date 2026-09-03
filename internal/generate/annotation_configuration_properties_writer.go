package generate

import (
	"bytes"
	"strconv"
)

func addConfigurationPropertiesImports(ctx *AnnotationGenerationContext, properties []annotationConfigurationProperties) {
	for _, item := range properties {
		for _, importSpec := range item.Imports {
			ctx.AddImport(importSpec.Alias, importSpec.Path)
		}
	}
}

func writeConfigurationProperties(builder *bytes.Buffer, properties annotationConfigurationProperties) {
	builder.WriteString("// Bind")
	builder.WriteString(properties.TypeName)
	builder.WriteString(" 从 Environment 绑定配置属性。\nfunc Bind")
	builder.WriteString(properties.TypeName)
	builder.WriteString("(environment goark.Environment) (out *")
	builder.WriteString(properties.TypeName)
	builder.WriteString(", err error) {\nout = &")
	builder.WriteString(properties.TypeName)
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
			builder.WriteString(strconv.Quote(field.Name))
			builder.WriteByte(',')
		}
		builder.WriteString("}); err != nil {\nreturn nil, err\n}\n")
	}
	builder.WriteString("if validator, ok := any(out).(goark.ConfigurationPropertiesValidator); ok {\nif err = validator.Validate(); err != nil {\nreturn nil, err\n}\n}\nreturn out, nil\n}\n\n")
	writeConfigurationPropertiesMetadata(builder, properties)
}

func writeConfigurationPropertyBinding(builder *bytes.Buffer, field annotationConfigurationPropertyField) {
	builder.WriteString("if value, found, bindErr := coreenv.GetPropertyAsValue[")
	builder.WriteString(field.Type)
	builder.WriteString("](environment, ")
	builder.WriteString(strconv.Quote(field.Name))
	builder.WriteString("); bindErr != nil {\nreturn nil, arkerrors.Wrapf(arkerrors.CodeConversion, bindErr, \"failed to bind configuration property %s\", ")
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
		builder.WriteString(" else {\nreturn nil, arkerrors.Newf(arkerrors.CodeNotFound, \"required configuration property %q not found\", ")
		builder.WriteString(strconv.Quote(field.Name))
		builder.WriteString(")\n}")
	}
	builder.WriteByte('\n')
}

func writeConfigurationPropertiesMetadata(builder *bytes.Buffer, properties annotationConfigurationProperties) {
	builder.WriteString("// ")
	builder.WriteString(properties.TypeName)
	builder.WriteString("ConfigurationMetadata 返回生成的配置属性元数据。\nfunc ")
	builder.WriteString(properties.TypeName)
	builder.WriteString("ConfigurationMetadata() []goark.ConfigurationProperty {\nreturn []goark.ConfigurationProperty{\n")
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

func writeConfigurationPropertiesRegistration(builder *bytes.Buffer, properties annotationConfigurationProperties) {
	builder.WriteString("if err := container.Register(registry, ")
	builder.WriteString(strconv.Quote(properties.BeanName))
	builder.WriteString(", func(_ context.Context, _ container.Resolver) (*")
	builder.WriteString(properties.TypeName)
	builder.WriteString(", error) {\nreturn Bind")
	builder.WriteString(properties.TypeName)
	builder.WriteString("(config.Environment())\n}); err != nil {\nreturn err\n}\n")
}
