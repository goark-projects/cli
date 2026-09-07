package generate

import (
	"fmt"
	"go/ast"
	"strings"
	"unicode"
	"unicode/utf8"
)

const sourcePackageAlias = "goarksource"

const (
	mvcErrParameterGroup   = "parameter group must declare exactly one name"
	mvcErrMultipleContexts = "must not declare multiple *arkarta/web.Context parameters"
	mvcErrParameterType    = "parameter must be *arkarta/web.Context or error type"
	mvcErrMultipleErrors   = "must declare exactly one error parameter"
	mvcErrErrorSelector    = "selector %q must reference error parameter"
	mvcErrReturnType       = "must return arkarta/web.Result, web.ResponseEntity, or ordinary value"
)

func prepareExternalGeneration(pkg *annotationPackage, values map[string]any) error {
	if pkg.SourceImportPath == "" {
		return nil
	}
	if err := validateExternalSymbols(pkg, values); err != nil {
		return err
	}
	qualifyCoreModel(pkg, values)
	qualifyWebModel(pkg, values)
	qualifyMVCModel(pkg, values)
	return nil
}

func validateExternalSymbols(pkg *annotationPackage, values map[string]any) error {
	for name := range pkg.types {
		if !ast.IsExported(name) && annotatedTypeName(values, name) {
			return fmt.Errorf("生成 gen 包要求注解类型 %s 可导出", name)
		}
	}
	model, _ := values[coreAnnotationModelKey].(*coreAnnotationModel)
	if model == nil {
		return nil
	}
	for _, configuration := range model.Configurations {
		for _, component := range configuration.Components {
			for _, field := range component.Fields {
				if !ast.IsExported(field.Name) {
					return fmt.Errorf("生成 gen 包要求注入字段 %s.%s 可导出", component.TypeName, field.Name)
				}
			}
		}
		for _, bean := range configuration.Beans {
			if !ast.IsExported(bean.MethodName) {
				return fmt.Errorf(
					"生成 gen 包要求 Bean 方法 %s.%s 可导出",
					configuration.TypeName,
					bean.MethodName,
				)
			}
		}
	}
	web, _ := values[webAnnotationModelKey].(*webAnnotationModel)
	if web != nil {
		for _, item := range webModelComponents(web) {
			if err := validateExternalComponent(item.Component); err != nil {
				return err
			}
		}
	}
	mvcModel, _ := values[mvcAnnotationModelKey].(*mvcAnnotationModel)
	if mvcModel != nil {
		for _, controller := range mvcModel.Controllers {
			if err := validateExternalComponent(controller.Component); err != nil {
				return err
			}
			for _, route := range controller.Routes {
				if !ast.IsExported(route.MethodName) {
					return fmt.Errorf("生成 gen 包要求 MVC 方法 %s 可导出", route.MethodName)
				}
			}
			for _, attribute := range controller.ModelAttributes {
				if !ast.IsExported(attribute.MethodName) {
					return fmt.Errorf("生成 gen 包要求 MVC 方法 %s 可导出", attribute.MethodName)
				}
			}
		}
		for _, advice := range mvcModel.Advices {
			if err := validateExternalComponent(advice.Component); err != nil {
				return err
			}
			for _, handler := range advice.ExceptionHandlers {
				if !ast.IsExported(handler.MethodName) {
					return fmt.Errorf("生成 gen 包要求异常处理方法 %s 可导出", handler.MethodName)
				}
			}
		}
	}
	return nil
}

func validateExternalComponent(component annotationComponent) error {
	for _, field := range component.Fields {
		if !ast.IsExported(field.Name) {
			return fmt.Errorf("生成 gen 包要求注入字段 %s.%s 可导出", component.TypeName, field.Name)
		}
	}
	return nil
}

func annotatedTypeName(values map[string]any, name string) bool {
	core, _ := values[coreAnnotationModelKey].(*coreAnnotationModel)
	if core != nil {
		for _, item := range core.Configurations {
			if item.TypeName == name {
				return true
			}
			for _, component := range item.Components {
				if component.TypeName == name {
					return true
				}
			}
		}
		for _, item := range core.ConfigurationProperties {
			if item.TypeName == name {
				return true
			}
		}
	}
	web, _ := values[webAnnotationModelKey].(*webAnnotationModel)
	if web != nil {
		for _, item := range webModelComponents(web) {
			if item.Component.TypeName == name {
				return true
			}
		}
	}
	mvcModel, _ := values[mvcAnnotationModelKey].(*mvcAnnotationModel)
	if mvcModel != nil {
		for _, item := range mvcModel.Controllers {
			if item.Component.TypeName == name {
				return true
			}
		}
		for _, item := range mvcModel.Advices {
			if item.Component.TypeName == name {
				return true
			}
		}
	}
	return false
}

func qualifyCoreModel(pkg *annotationPackage, values map[string]any) {
	model, _ := values[coreAnnotationModelKey].(*coreAnnotationModel)
	if model == nil {
		return
	}
	for index := range model.ConfigurationProperties {
		properties := &model.ConfigurationProperties[index]
		qualifyProperties(pkg, properties)
	}
	for _, configuration := range model.Configurations {
		if !configuration.Synthetic {
			configuration.SourceTypeName = qualifyLocalIdentifiers(
				configuration.TypeName,
				pkg.types,
			)
			configuration.Synthetic = true
		}
		qualifyConfiguration(pkg, configuration)
	}
}

func qualifyConfiguration(pkg *annotationPackage, configuration *annotationConfiguration) {
	for index := range configuration.Properties {
		qualifyProperties(pkg, &configuration.Properties[index])
	}
	for index := range configuration.Components {
		qualifyComponent(pkg, &configuration.Components[index])
	}
	for index := range configuration.Beans {
		bean := &configuration.Beans[index]
		bean.ReturnType = qualifyLocalIdentifiers(bean.ReturnType, pkg.types)
		bean.Condition = qualifyLocalIdentifiers(bean.Condition, pkg.types)
		bean.ConfigurationType = configuration.SourceTypeName
		for item := range bean.Params {
			bean.Params[item].Type = qualifyLocalIdentifiers(bean.Params[item].Type, pkg.types)
		}
	}
}

func qualifyProperties(pkg *annotationPackage, properties *annotationConfigurationProperties) {
	properties.SourceTypeName = qualifyLocalIdentifiers(properties.TypeName, pkg.types)
	for item := range properties.Initializers {
		properties.Initializers[item] = qualifyLocalIdentifiers(
			properties.Initializers[item],
			pkg.types,
		)
	}
	for item := range properties.Fields {
		properties.Fields[item].Type = qualifyLocalIdentifiers(
			properties.Fields[item].Type,
			pkg.types,
		)
		properties.Fields[item].MapValueType = qualifyLocalIdentifiers(
			properties.Fields[item].MapValueType,
			pkg.types,
		)
	}
}

func qualifyComponent(pkg *annotationPackage, component *annotationComponent) {
	component.TypeName = qualifyLocalIdentifiers(component.TypeName, pkg.types)
	component.Condition = qualifyLocalIdentifiers(component.Condition, pkg.types)
	for index := range component.Fields {
		component.Fields[index].Type = qualifyLocalIdentifiers(
			component.Fields[index].Type,
			pkg.types,
		)
	}
}

func qualifyWebModel(pkg *annotationPackage, values map[string]any) {
	model, _ := values[webAnnotationModelKey].(*webAnnotationModel)
	if model == nil {
		return
	}
	for _, item := range webModelComponents(model) {
		qualifyComponent(pkg, &item.Component)
	}
}

func qualifyMVCModel(pkg *annotationPackage, values map[string]any) {
	model, _ := values[mvcAnnotationModelKey].(*mvcAnnotationModel)
	if model == nil {
		return
	}
	for _, controller := range model.Controllers {
		qualifyComponent(pkg, &controller.Component)
		for index := range controller.Routes {
			qualifyMVCRoute(pkg, &controller.Routes[index])
		}
		for index := range controller.ModelAttributes {
			attribute := &controller.ModelAttributes[index]
			attribute.ReturnType = qualifyLocalIdentifiers(attribute.ReturnType, pkg.types)
			qualifyMVCParams(pkg, attribute.Params)
		}
	}
	for _, advice := range model.Advices {
		qualifyComponent(pkg, &advice.Component)
		for index := range advice.ExceptionHandlers {
			handler := &advice.ExceptionHandlers[index]
			handler.ErrorType = qualifyLocalIdentifiers(handler.ErrorType, pkg.types)
			handler.EntityBody = qualifyLocalIdentifiers(handler.EntityBody, pkg.types)
		}
	}
}

func qualifyMVCRoute(pkg *annotationPackage, route *mvcRoute) {
	route.Handler.ReturnType = qualifyLocalIdentifiers(route.Handler.ReturnType, pkg.types)
	route.Handler.EntityBody = qualifyLocalIdentifiers(route.Handler.EntityBody, pkg.types)
	qualifyMVCParams(pkg, route.Handler.Params)
}

func qualifyMVCParams(pkg *annotationPackage, params []mvcHandlerParam) {
	for index := range params {
		params[index].Type = qualifyLocalIdentifiers(params[index].Type, pkg.types)
		params[index].BodyType = qualifyLocalIdentifiers(params[index].BodyType, pkg.types)
	}
}

func qualifyLocalIdentifiers(value string, types map[string]annotationTypeDeclaration) string {
	if value == "" {
		return value
	}
	var builder strings.Builder
	builder.Grow(len(value) + len(sourcePackageAlias) + 1)
	for index := 0; index < len(value); {
		current, size := utf8.DecodeRuneInString(value[index:])
		if current != '_' && !unicode.IsLetter(current) {
			builder.WriteString(value[index : index+size])
			index += size
			continue
		}
		start := index
		for index < len(value) {
			current, size = utf8.DecodeRuneInString(value[index:])
			if current != '_' && !unicode.IsLetter(current) && !unicode.IsDigit(current) {
				break
			}
			index += size
		}
		name := value[start:index]
		previousIsSelector := start > 0 && value[start-1] == '.'
		if _, ok := types[name]; ok && !previousIsSelector {
			builder.WriteString(sourcePackageAlias)
			builder.WriteByte('.')
		}
		builder.WriteString(name)
	}
	return builder.String()
}

func annotationError(format string, args ...any) error {
	return fmt.Errorf("annotation "+"%q "+format, args...)
}

func mvcUnsupportedBodyReturnError(method string, kind string) error {
	return mvcHandlerError("with %s must return T, T,error, web.ResponseEntity, or "+
		"web.ResponseEntity,error", method, kind)
}

func mvcHandlerError(format string, args ...any) error {
	return fmt.Errorf("mvc handler method "+"%s "+format, args...)
}

func mvcMissingSelectorError(method string, kind string) error {
	return mvcHandlerError(kind+" selector does not match any method parameter", method)
}

func mvcSelectorError(method string, kind string, selector string) error {
	return mvcHandlerError(
		kind+" selector %q does not match any method parameter", method, selector,
	)
}

func mvcContextBindingError(method string, kind string, parameter string) error {
	return mvcHandlerError(
		"%s parameter %s must not be *arkarta/web.Context", method, kind, parameter,
	)
}

func mvcModelBindingError(method string, kind string, parameter string) error {
	return mvcHandlerError("%s parameter %s must not be *mvc.Model", method, kind, parameter)
}

func mvcMultipleBindingError(method string, parameter string) error {
	return mvcHandlerError(
		"parameter %s must not declare multiple mvc binding annotations", method, parameter,
	)
}

func mvcExceptionHandlerError(method string, format string, args ...any) error {
	values := make([]any, 0, len(args)+1)
	values = append(values, method)
	values = append(values, args...)
	return fmt.Errorf("mvc exception handler method %s "+format, values...)
}

func validateMVCHandlerMethod(ctx AnnotationValidationContext) error {
	fn := ctx.Item.FuncDecl()
	if fn == nil || fn.Recv == nil {
		return annotationError("requires concrete method with receiver", ctx.Annotation.Name)
	}
	if ctx.Item.ReceiverTypeName() == "" {
		return annotationError("receiver is not supported", ctx.Annotation.Name)
	}
	return nil
}
