package generate

import (
	"bytes"
	"go/ast"
	"strconv"
	"strings"

	"goark.dev/cli/internal/generate/mvcrouting"
)

func mvcParameterKind(name string) (mvcHandlerParamKind, bool) {
	switch name {
	case "goark-web:path-variable":
		return mvcParamPathVariable, true
	case "goark-web:request-param":
		return mvcParamRequestParam, true
	case "goark-web:request-header":
		return mvcParamRequestHeader, true
	case "goark-web:cookie-value":
		return mvcParamCookieValue, true
	case "goark-web:model-attribute":
		return mvcParamModelAttribute, true
	case "goark-web:request-attribute":
		return mvcParamRequestAttribute, true
	case "goark-web:session-attribute":
		return mvcParamSessionAttribute, true
	case "goark-web:matrix-variable":
		return mvcParamMatrixVariable, true
	case "goark-web:request-part":
		return mvcParamRequestPart, true
	default:
		return 0, false
	}
}

func mvcBindingSelector(annotation Annotation) string {
	selector := normalizeSelector(annotation.Selector)
	if selector != "" {
		return selector
	}
	return strings.TrimSpace(argString(annotation, "param", ""))
}

func mvcModelAttributeMethodNameAnnotation(annotations []Annotation) string {
	for _, annotation := range annotations {
		if annotation.Name == "goark-web:model-attribute" {
			return mvcModelAttributeMethodName(annotation)
		}
	}
	return ""
}

func mvcModelAttributeAnnotationCount(annotations []Annotation) int {
	count := 0
	for _, annotation := range annotations {
		if annotation.Name == "goark-web:model-attribute" {
			count++
		}
	}
	return count
}

func mvcModelAttributeMethodName(annotation Annotation) string {
	values := annotationValueTexts(annotation)
	if len(values) == 1 {
		if value := strings.TrimSpace(values[0]); value != "" {
			return value
		}
	}
	for _, key := range []string{"name", "value"} {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			return value
		}
	}
	return ""
}

func mvcParameterSourceName(annotation Annotation, fallback string) string {
	values := annotationValueTexts(annotation)
	if len(values) == 1 {
		if value := strings.TrimSpace(values[0]); value != "" {
			return value
		}
	}
	for _, key := range []string{"name", "value"} {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			return value
		}
	}
	return fallback
}

func mvcParameterHasSourceName(annotation Annotation) bool {
	if len(annotationValueTexts(annotation)) > 0 {
		return true
	}
	for _, key := range []string{"name", "value"} {
		if strings.TrimSpace(argString(annotation, key, "")) != "" {
			return true
		}
	}
	return false
}

func mvcParameterRequired(annotation Annotation) bool {
	if mvcParameterHasDefault(annotation) {
		return false
	}
	return annotationBool(annotation, "required", true)
}

func mvcParameterHasDefault(annotation Annotation) bool {
	_, ok := annotation.Args["defaultValue"]
	if ok {
		return true
	}
	_, ok = annotation.Args["default"]
	return ok
}

func mvcParameterDefaultValue(annotation Annotation) string {
	if value, ok := annotation.Args["defaultValue"]; ok {
		return value.Text()
	}
	if value, ok := annotation.Args["default"]; ok {
		return value.Text()
	}
	return ""
}

func mvcHasParam(params []mvcHandlerParam, name string) bool {
	for _, param := range params {
		if param.Name == name {
			return true
		}
	}
	return false
}

func hasMVCBodyParam(params []mvcHandlerParam) bool {
	_, ok := mvcBodyParam(params)
	return ok
}

func hasMVCMultipartBodyParam(params []mvcHandlerParam) bool {
	_, ok := mvcMultipartBodyParam(params)
	return ok
}

func hasMVCModelAttributeParam(params []mvcHandlerParam) bool {
	for _, param := range params {
		if param.Kind == mvcParamModelAttribute {
			return true
		}
	}
	return false
}

func hasMVCModelParam(params []mvcHandlerParam) bool {
	_, ok := mvcModelParam(params)
	return ok
}

func mvcBodyParam(params []mvcHandlerParam) (mvcHandlerParam, bool) {
	for _, param := range params {
		if param.Kind == mvcParamBody {
			return param, true
		}
	}
	return mvcHandlerParam{}, false
}

func mvcMultipartBodyParam(params []mvcHandlerParam) (mvcHandlerParam, bool) {
	for _, param := range params {
		if param.Kind == mvcParamMultipartBody {
			return param, true
		}
	}
	return mvcHandlerParam{}, false
}

func mvcModelParam(params []mvcHandlerParam) (mvcHandlerParam, bool) {
	for _, param := range params {
		if param.Kind == mvcParamModel {
			return param, true
		}
	}
	return mvcHandlerParam{}, false
}

func isMVCModelAttributeTypeExpr(expr ast.Expr) bool {
	switch typ := expr.(type) {
	case *ast.StarExpr, *ast.ArrayType, *ast.MapType, *ast.InterfaceType, *ast.FuncType, *ast.ChanType:
		return false
	case *ast.Ident:
		return !isMVCScalarTypeName(typ.Name)
	case *ast.SelectorExpr:
		return true
	default:
		return false
	}
}

func isMVCScalarTypeName(name string) bool {
	switch strings.TrimSpace(name) {
	case "string", "bool",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128",
		"byte", "rune", "any", "error":
		return true
	default:
		return false
	}
}

func writeMVCConfiguration(builder *bytes.Buffer, model *mvcAnnotationModel) {
	builder.WriteString("type GoarkWebMVCConfiguration struct{}\n\n")
	builder.WriteString("func (GoarkWebMVCConfiguration) Name() string {\n" +
		"return \"goark.web.mvc\"\n}\n\n")
	builder.WriteString("func (GoarkWebMVCConfiguration) Order() int {\nreturn 0\n}\n\n")
	builder.WriteString("func (c GoarkWebMVCConfiguration) Register(ctx context.Context, " +
		"registry *container.Registry) error {\n")
	builder.WriteString("return c.RegisterWithContext(ctx, " +
		"goark.NewConfigurationContext(nil, registry))\n")
	builder.WriteString("}\n\n")
	builder.WriteString("func (c GoarkWebMVCConfiguration) RegisterWithContext(ctx context.Context, " +
		"config goark.ConfigurationContext) error {\n")
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
	builder.WriteString(", func(ctx context.Context, resolver container.Resolver) " +
		"(out goweb.Configurer, err error) {\n")
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
	builder.WriteString(mvcrouting.Constructor(route.HTTPMethod))
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
	if kind == "goark-web:rest-controller" {
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
	if route.ResponseBody || route.ControllerKind == "goark-web:rest-controller" ||
		!hasMVCModelParam(route.Handler.Params) {
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
