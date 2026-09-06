package generate

import (
	"bytes"
	"strconv"
	"strings"
)

func writeMVCConfiguration(builder *bytes.Buffer, model *mvcAnnotationModel) {
	builder.WriteString("type GoarkWebMVCConfiguration struct{}\n\n")
	builder.WriteString("func (GoarkWebMVCConfiguration) Name() string {\nreturn \"goark.web.mvc\"\n}\n\n")
	builder.WriteString("func (GoarkWebMVCConfiguration) Order() int {\nreturn 0\n}\n\n")
	builder.WriteString("func (c GoarkWebMVCConfiguration) Register(ctx context.Context, registry *container.Registry) error {\n")
	builder.WriteString("return c.RegisterWithContext(ctx, goark.NewConfigurationContext(nil, registry))\n")
	builder.WriteString("}\n\n")
	builder.WriteString("func (c GoarkWebMVCConfiguration) RegisterWithContext(ctx context.Context, config goark.ConfigurationContext) error {\n")
	builder.WriteString("registry := config.Registry()\n")
	for _, controller := range model.Controllers {
		writeComponentRegistration(builder, controller.Component)
		writeMVCConfigurerRegistration(builder, controller)
	}
	for _, advice := range model.Advices {
		writeComponentRegistration(builder, advice.Component)
		writeMVCAdviceConfigurerRegistration(builder, advice)
	}
	builder.WriteString("return nil\n}\n\n")
}

func writeMVCConfigurerRegistration(builder *bytes.Buffer, controller *mvcController) {
	configurerName := controller.Component.Name + ".mvcConfigurer"
	builder.WriteString("if err := container.Register[goweb.Configurer](registry, ")
	builder.WriteString(strconv.Quote(configurerName))
	builder.WriteString(", func(ctx context.Context, resolver container.Resolver) (out goweb.Configurer, err error) {\n")
	builder.WriteString("controller, err := container.GetByType[*")
	builder.WriteString(controller.Component.TypeName)
	builder.WriteString("](ctx, resolver, container.WithQualifier(")
	builder.WriteString(strconv.Quote(controller.Component.Name))
	builder.WriteString("))\nif err != nil {\nreturn nil, err\n}\n")
	builder.WriteString("out = mvc.NewConfigurer(")
	builder.WriteString(mvcControllerConstructor(controller.Kind))
	builder.WriteByte('(')
	builder.WriteString(strconv.Quote(controller.Component.Name))
	for _, route := range controller.Routes {
		builder.WriteString(",\n")
		writeMVCRoute(builder, route)
	}
	builder.WriteByte(')')
	writeMVCControllerOptions(builder, controller.Conditions)
	writeMVCControllerCrossOrigin(builder, controller.CrossOrigin)
	writeMVCModelAttributeMethods(builder, controller.ModelAttributes)
	builder.WriteString(")\nreturn out, nil\n}, container.WithFactoryDependencies(")
	builder.WriteString(strconv.Quote(controller.Component.Name))
	builder.WriteString(")); err != nil {\nreturn err\n}\n")
}

func writeMVCRoute(builder *bytes.Buffer, route mvcRoute) {
	builder.WriteString("mvc.")
	builder.WriteString(routeConstructor(route.HTTPMethod))
	builder.WriteByte('(')
	builder.WriteString(strconv.Quote(route.Path))
	builder.WriteString(", ")
	writeMVCHandler(builder, route)
	writeMVCRouteOptions(builder, route.Conditions)
	writeMVCCrossOriginRouteOption(builder, route.CrossOrigin)
	builder.WriteByte(')')
}

func writeMVCModelAttributeMethods(builder *bytes.Buffer, attributes []mvcModelAttributeMethod) {
	if len(attributes) == 0 {
		return
	}
	builder.WriteString(".WithModelAttributes(")
	for index, attribute := range attributes {
		if index > 0 {
			builder.WriteString(",\n")
		}
		writeMVCModelAttributeMethod(builder, attribute)
	}
	builder.WriteByte(')')
}

func writeMVCModelAttributeMethod(builder *bytes.Buffer, attribute mvcModelAttributeMethod) {
	builder.WriteString("mvc.ModelAttributeValue[")
	builder.WriteString(attribute.ReturnType)
	builder.WriteString("](")
	builder.WriteString(strconv.Quote(attribute.Name))
	builder.WriteString(", func(ctx *arkweb.Context) (")
	builder.WriteString(attribute.ReturnType)
	builder.WriteString(", error) {\nreturn ")
	builder.WriteString(mvcHandlerCall(attribute.MethodName, attribute.Params))
	if attribute.ReturnKind == mvcReturnValue {
		builder.WriteString(", nil")
	}
	builder.WriteString("\n})")
}

func mvcControllerConstructor(kind string) string {
	if kind == "rest-controller" {
		return "mvc.NewRestController"
	}
	return "mvc.NewController"
}

func writeMVCHandler(builder *bytes.Buffer, route mvcRoute) {
	if shouldRenderMVCModelView(route) {
		writeMVCModelViewHandler(builder, route)
		return
	}
	if shouldWrapMVCResponseStatus(route) {
		builder.WriteString("mvc.ResponseStatus(")
		builder.WriteString(strconv.Itoa(route.Status))
		builder.WriteString(", ")
		writeMVCHandlerCore(builder, route)
		builder.WriteByte(')')
		return
	}
	writeMVCHandlerCore(builder, route)
}

func shouldRenderMVCModelView(route mvcRoute) bool {
	if route.ResponseBody || route.ControllerKind == "rest-controller" || !hasMVCModelParam(route.Handler.Params) {
		return false
	}
	switch route.Handler.ReturnKind {
	case mvcReturnNone, mvcReturnError:
		return true
	case mvcReturnValue, mvcReturnValueError:
		return strings.TrimSpace(route.Handler.ReturnType) == "string"
	default:
		return false
	}
}

func shouldWrapMVCResponseStatus(route mvcRoute) bool {
	if !route.StatusExplicit {
		return false
	}
	switch route.Handler.ReturnKind {
	case mvcReturnNone, mvcReturnError, mvcReturnResult, mvcReturnResultError:
		return true
	default:
		return false
	}
}

func writeMVCModelViewHandler(builder *bytes.Buffer, route mvcRoute) {
	modelParam, _ := mvcModelParam(route.Handler.Params)
	call := mvcHandlerCall(route.MethodName, route.Handler.Params)
	builder.WriteString("mvc.Handler(func(ctx *arkweb.Context) (arkweb.Result, error) {\n")
	writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err", route.ValidationGroups)
	switch route.Handler.ReturnKind {
	case mvcReturnError:
		builder.WriteString("if err := ")
		builder.WriteString(call)
		builder.WriteString("; err != nil {\nreturn nil, err\n}\n")
		writeMVCModelAndViewReturn(builder, "", modelParam.Name, route.Status)
	case mvcReturnValue:
		builder.WriteString("viewName := ")
		builder.WriteString(call)
		builder.WriteByte('\n')
		writeMVCModelAndViewReturn(builder, "viewName", modelParam.Name, route.Status)
	case mvcReturnValueError:
		builder.WriteString("viewName, err := ")
		builder.WriteString(call)
		builder.WriteString("\nif err != nil {\nreturn nil, err\n}\n")
		writeMVCModelAndViewReturn(builder, "viewName", modelParam.Name, route.Status)
	default:
		builder.WriteString(call)
		builder.WriteByte('\n')
		writeMVCModelAndViewReturn(builder, "", modelParam.Name, route.Status)
	}
	builder.WriteString("\n})")
}

func writeMVCModelAndViewReturn(builder *bytes.Buffer, viewName string, modelName string, statusCode int) {
	builder.WriteString("return mvc.NewModelAndView(")
	if viewName == "" {
		builder.WriteString(strconv.Quote(""))
	} else {
		builder.WriteString(viewName)
	}
	builder.WriteString(", ")
	builder.WriteString(modelName)
	builder.WriteString(", mvc.WithViewStatus(")
	builder.WriteString(strconv.Itoa(statusCode))
	builder.WriteString(")), nil")
}

func writeMVCHandlerCore(builder *bytes.Buffer, route mvcRoute) {
	if hasMVCBodyParam(route.Handler.Params) {
		if route.Handler.ReturnKind == mvcReturnEntity || route.Handler.ReturnKind == mvcReturnEntityError {
			writeMVCBindEntityHandler(builder, route)
			return
		}
		writeMVCBindJSONHandler(builder, route)
		return
	}
	if hasMVCRequestEntityParam(route.Handler.Params) {
		if route.Handler.ReturnKind == mvcReturnEntity || route.Handler.ReturnKind == mvcReturnEntityError {
			writeMVCBindRequestEntityEntityHandler(builder, route)
			return
		}
		writeMVCBindRequestEntityHandler(builder, route)
		return
	}
	if hasMVCMultipartBodyParam(route.Handler.Params) {
		if route.Handler.ReturnKind == mvcReturnEntity || route.Handler.ReturnKind == mvcReturnEntityError {
			writeMVCBindMultipartEntityHandler(builder, route)
			return
		}
		writeMVCBindMultipartHandler(builder, route)
		return
	}
	call := mvcHandlerCall(route.MethodName, route.Handler.Params)
	switch route.Handler.ReturnKind {
	case mvcReturnResultError, mvcReturnEntityError:
		builder.WriteString("mvc.Handler(func(ctx *arkweb.Context) (arkweb.Result, error) {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err", route.ValidationGroups)
		builder.WriteString("return ")
		builder.WriteString(call)
		builder.WriteString("\n})")
	case mvcReturnResult, mvcReturnEntity:
		builder.WriteString("mvc.Handler(func(ctx *arkweb.Context) (arkweb.Result, error) {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err", route.ValidationGroups)
		builder.WriteString("return ")
		builder.WriteString(call)
		builder.WriteString(", nil\n})")
	case mvcReturnValueError:
		writeMVCValueReturnHandlerName(builder, route)
		builder.WriteString(strconv.Itoa(route.Status))
		builder.WriteString(", func(ctx *arkweb.Context) (any, error) {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err", route.ValidationGroups)
		builder.WriteString("return ")
		builder.WriteString(call)
		builder.WriteString("\n})")
	case mvcReturnValue:
		writeMVCValueReturnHandlerName(builder, route)
		builder.WriteString(strconv.Itoa(route.Status))
		builder.WriteString(", func(ctx *arkweb.Context) (any, error) {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err", route.ValidationGroups)
		builder.WriteString("return ")
		builder.WriteString(call)
		builder.WriteString(", nil\n})")
	case mvcReturnError:
		builder.WriteString("mvc.NoContent(func(ctx *arkweb.Context) error {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return err", route.ValidationGroups)
		builder.WriteString("return ")
		builder.WriteString(call)
		builder.WriteString("\n})")
	default:
		builder.WriteString("mvc.NoContent(func(ctx *arkweb.Context) error {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return err", route.ValidationGroups)
		builder.WriteString(call)
		builder.WriteString("\nreturn nil\n})")
	}
}

func writeMVCValueReturnHandlerName(builder *bytes.Buffer, route mvcRoute) {
	if route.ResponseBody {
		builder.WriteString("mvc.ResponseBody[any](")
		return
	}
	builder.WriteString("mvc.Return[any](")
}
