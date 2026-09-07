package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

func mvcModelUsesOptionalInjection(model *mvcAnnotationModel) bool {
	for _, controller := range model.Controllers {
		for _, field := range controller.Component.Fields {
			if !field.Injection.Required && field.Injection.Kind != "value" {
				return true
			}
		}
	}
	for _, advice := range model.Advices {
		for _, field := range advice.Component.Fields {
			if !field.Injection.Required && field.Injection.Kind != "value" {
				return true
			}
		}
	}
	return false
}

func mvcModelUsesArkWeb(model *mvcAnnotationModel) bool {
	for _, controller := range model.Controllers {
		if len(controller.Routes) > 0 || len(controller.ModelAttributes) > 0 {
			return true
		}
	}
	for _, advice := range model.Advices {
		if len(advice.ExceptionHandlers) > 0 {
			return true
		}
	}
	return false
}

func buildMVCControllerAdvice(
	fset *token.FileSet, typeSpec *ast.TypeSpec, annotations []Annotation,
) (*mvcControllerAdvice, error) {
	component, err := buildMVCComponent(
		fset, typeSpec, annotations, mvcControllerAdviceKind(annotations),
	)
	if err != nil {
		return nil, err
	}
	return &mvcControllerAdvice{
		Component: component,
		Kind:      mvcControllerAdviceKind(annotations),
	}, nil
}

func writeMVCAdviceConfigurerRegistration(builder *bytes.Buffer, advice *mvcControllerAdvice) {
	if len(advice.ExceptionHandlers) == 0 {
		return
	}
	configurerName := advice.Component.Name + ".mvcAdviceConfigurer"
	builder.WriteString("if err := container.Register[goweb.Configurer](registry, ")
	builder.WriteString(strconv.Quote(configurerName))
	builder.WriteString(", func(ctx context.Context, resolver container.Resolver) " +
		"(out goweb.Configurer, err error) {\n")
	builder.WriteString("advice, err := container.GetByType[*")
	builder.WriteString(advice.Component.TypeName)
	builder.WriteString("](ctx, resolver, container.WithQualifier(")
	builder.WriteString(strconv.Quote(advice.Component.Name))
	builder.WriteString("))\nif err != nil {\nreturn nil, err\n}\n")
	builder.WriteString("out = mvc.NewConfigurer().WithControllerAdvices(")
	builder.WriteString(mvcControllerAdviceConstructor(advice.Kind))
	builder.WriteByte('(')
	builder.WriteString(strconv.Quote(advice.Component.Name))
	for _, handler := range advice.ExceptionHandlers {
		builder.WriteString(",\n")
		writeMVCExceptionHandler(builder, handler)
	}
	builder.WriteString("))\nreturn out, nil\n}, container.WithFactoryDependencies(")
	builder.WriteString(strconv.Quote(advice.Component.Name))
	builder.WriteString(")); err != nil {\nreturn err\n}\n")
}

func mvcControllerAdviceConstructor(kind string) string {
	if kind == "rest-controller-advice" {
		return "mvc.NewRestControllerAdvice"
	}
	return "mvc.NewControllerAdvice"
}

func writeMVCExceptionHandler(builder *bytes.Buffer, handler mvcExceptionHandler) {
	switch handler.ReturnKind {
	case mvcReturnEntity:
		writeMVCExceptionEntityHandler(builder, handler)
	case mvcReturnValue:
		writeMVCExceptionValueHandler(builder, handler)
	default:
		writeMVCExceptionResultHandler(builder, handler)
	}
}

func writeMVCExceptionResultHandler(builder *bytes.Buffer, handler mvcExceptionHandler) {
	builder.WriteString("mvc.ExceptionHandlerAs[")
	builder.WriteString(handler.ErrorType)
	builder.WriteString("](func(")
	if mvcExceptionHandlerUsesContext(handler) {
		builder.WriteString("ctx")
	} else {
		builder.WriteString("_")
	}
	builder.WriteString(" *arkweb.Context, err ")
	builder.WriteString(handler.ErrorType)
	builder.WriteString(") arkweb.Result {\nreturn advice.")
	builder.WriteString(handler.MethodName)
	builder.WriteByte('(')
	builder.WriteString(mvcExceptionHandlerCallArgs(handler.Params))
	builder.WriteString(")\n})")
}

func writeMVCExceptionEntityHandler(builder *bytes.Buffer, handler mvcExceptionHandler) {
	builder.WriteString("mvc.ExceptionEntityAs[")
	builder.WriteString(handler.ErrorType)
	builder.WriteString(", ")
	builder.WriteString(handler.EntityBody)
	builder.WriteString("](func(")
	if mvcExceptionHandlerUsesContext(handler) {
		builder.WriteString("ctx")
	} else {
		builder.WriteString("_")
	}
	builder.WriteString(" *arkweb.Context, err ")
	builder.WriteString(handler.ErrorType)
	builder.WriteString(") goweb.ResponseEntity[")
	builder.WriteString(handler.EntityBody)
	builder.WriteString("] {\nreturn advice.")
	builder.WriteString(handler.MethodName)
	builder.WriteByte('(')
	builder.WriteString(mvcExceptionHandlerCallArgs(handler.Params))
	builder.WriteString(")\n})")
}

func writeMVCExceptionValueHandler(builder *bytes.Buffer, handler mvcExceptionHandler) {
	if handler.ResponseBody {
		builder.WriteString("mvc.ExceptionResponseBodyAs[")
	} else {
		builder.WriteString("mvc.ExceptionReturnAs[")
	}
	builder.WriteString(handler.ErrorType)
	builder.WriteString(", any](")
	builder.WriteString(strconv.Itoa(handler.Status))
	builder.WriteString(", func(")
	if mvcExceptionHandlerUsesContext(handler) {
		builder.WriteString("ctx")
	} else {
		builder.WriteString("_")
	}
	builder.WriteString(" *arkweb.Context, err ")
	builder.WriteString(handler.ErrorType)
	builder.WriteString(") any {\nreturn advice.")
	builder.WriteString(handler.MethodName)
	builder.WriteByte('(')
	builder.WriteString(mvcExceptionHandlerCallArgs(handler.Params))
	builder.WriteString(")\n})")
}

func mvcExceptionHandlerCallArgs(params []mvcExceptionHandlerParam) string {
	args := make([]string, 0, len(params))
	for _, param := range params {
		switch param.Kind {
		case mvcExceptionParamContext:
			args = append(args, "ctx")
		case mvcExceptionParamError:
			args = append(args, "err")
		}
	}
	return strings.Join(args, ", ")
}

func (mvcAnnotationBinder) BindAnnotation(
	ctx *AnnotationBindingContext,
	item AnnotationItem,
) error {
	switch item.Target() {
	case AnnotationTargetType:
		if err := bindMVCController(ctx, item); err != nil {
			return err
		}
		return bindMVCControllerAdvice(ctx, item)
	case AnnotationTargetMethod:
		if err := bindMVCRoute(ctx, item); err != nil {
			return err
		}
		if err := bindMVCModelAttributeMethod(ctx, item); err != nil {
			return err
		}
		return bindMVCExceptionHandler(ctx, item)
	default:
		return nil
	}
}

func (mvcAnnotationBinder) FinalizeAnnotationBinding(ctx *AnnotationBindingContext) error {
	value, ok := ctx.Value(mvcAnnotationModelKey)
	if !ok {
		return nil
	}
	model, ok := value.(*mvcAnnotationModel)
	if !ok {
		return fmt.Errorf("invalid mvc annotation model")
	}
	for _, route := range model.pending {
		controller := model.byType[route.ControllerType]
		if controller == nil {
			return fmt.Errorf(
				"mvc route method %s.%s requires mvc controller receiver type",
				route.ControllerType, route.MethodName,
			)
		}
		routes, err := expandMVCRoutePaths(controller, route)
		if err != nil {
			return err
		}
		controller.Routes = append(controller.Routes, routes...)
	}
	for _, attribute := range model.pendingModelAttributes {
		controller := model.byType[attribute.ControllerType]
		if controller == nil {
			return fmt.Errorf(
				"mvc model attribute method %s.%s requires mvc controller receiver type",
				attribute.ControllerType, attribute.MethodName,
			)
		}
		controller.ModelAttributes = append(controller.ModelAttributes, attribute)
	}
	for _, handler := range model.pendingExceptionHandlers {
		advice := model.adviceByType[handler.AdviceType]
		if advice == nil {
			return fmt.Errorf(
				"mvc exception handler method %s.%s requires mvc controller advice receiver type",
				handler.AdviceType, handler.MethodName,
			)
		}
		advice.ExceptionHandlers = append(advice.ExceptionHandlers, handler)
	}
	coreModel := ensureCoreAnnotationModel(ctx)
	resolver := newAnnotationDependencyResolver(coreModel)
	for _, controller := range model.Controllers {
		resolver.addCandidate(annotationDependencyCandidate{
			Name: controller.Component.Name,
			Type: "*" + controller.Component.TypeName,
		})
	}
	for _, advice := range model.Advices {
		resolver.addCandidate(annotationDependencyCandidate{
			Name: advice.Component.Name,
			Type: "*" + advice.Component.TypeName,
		})
	}
	for _, controller := range model.Controllers {
		inferComponentDependencyMetadata(&controller.Component, resolver)
		sort.SliceStable(controller.Routes, func(i, j int) bool {
			left := controller.Routes[i]
			right := controller.Routes[j]
			if left.Path == right.Path {
				if left.HTTPMethod == right.HTTPMethod {
					return left.MethodName < right.MethodName
				}
				return left.HTTPMethod < right.HTTPMethod
			}
			return left.Path < right.Path
		})
		sort.SliceStable(controller.ModelAttributes, func(i, j int) bool {
			left := controller.ModelAttributes[i]
			right := controller.ModelAttributes[j]
			if left.Name == right.Name {
				return left.MethodName < right.MethodName
			}
			return left.Name < right.Name
		})
	}
	for _, advice := range model.Advices {
		inferComponentDependencyMetadata(&advice.Component, resolver)
		sort.SliceStable(advice.ExceptionHandlers, func(i, j int) bool {
			left := advice.ExceptionHandlers[i]
			right := advice.ExceptionHandlers[j]
			if left.ErrorType == right.ErrorType {
				return left.MethodName < right.MethodName
			}
			return left.ErrorType < right.ErrorType
		})
	}
	sort.SliceStable(model.Controllers, func(i, j int) bool {
		return model.Controllers[i].Component.Name < model.Controllers[j].Component.Name
	})
	sort.SliceStable(model.Advices, func(i, j int) bool {
		return model.Advices[i].Component.Name < model.Advices[j].Component.Name
	})
	if len(model.Controllers) > 0 || len(model.Advices) > 0 {
		ctx.AddConfigurationType("GoarkWebMVCConfiguration")
	}
	return nil
}

func bindMVCController(ctx *AnnotationBindingContext, item AnnotationItem) error {
	if !hasMVCControllerAnnotation(item.Annotations()) {
		return nil
	}
	typeSpec := item.TypeSpec()
	if typeSpec == nil {
		return nil
	}
	controller, err := buildMVCController(item.FileSet(), typeSpec, item.Annotations())
	if err != nil {
		return err
	}
	model := ensureMVCAnnotationModel(ctx)
	if _, exists := model.byType[controller.Component.TypeName]; exists {
		return fmt.Errorf("duplicate mvc controller type %q", controller.Component.TypeName)
	}
	model.Controllers = append(model.Controllers, controller)
	model.byType[controller.Component.TypeName] = controller
	return nil
}

func bindMVCRoute(ctx *AnnotationBindingContext, item AnnotationItem) error {
	if !hasMVCRouteMappingAnnotation(item.Annotations()) {
		return nil
	}
	route, err := buildMVCRoute(item.FileSet(), item.File(), item.FuncDecl(), item.Annotations())
	if err != nil {
		return err
	}
	route.ControllerType = item.ReceiverTypeName()
	route.MethodName = item.FuncName()
	model := ensureMVCAnnotationModel(ctx)
	model.pending = append(model.pending, route)
	return nil
}

func bindMVCModelAttributeMethod(ctx *AnnotationBindingContext, item AnnotationItem) error {
	if hasMVCRouteMappingAnnotation(item.Annotations()) ||
		!hasAnnotation(item.Annotations(), "model-attribute") {
		return nil
	}
	attribute, err := buildMVCModelAttributeMethod(
		item.FileSet(), item.File(), item.FuncDecl(), item.Annotations(),
	)
	if err != nil {
		return err
	}
	attribute.ControllerType = item.ReceiverTypeName()
	attribute.MethodName = item.FuncName()
	model := ensureMVCAnnotationModel(ctx)
	model.pendingModelAttributes = append(model.pendingModelAttributes, attribute)
	return nil
}
