package generate

import (
	"fmt"
	"sort"
)

func (mvcAnnotationBinder) BindAnnotation(ctx *AnnotationBindingContext, item AnnotationItem) error {
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
			return fmt.Errorf("mvc route method %s.%s requires mvc controller receiver type", route.ControllerType, route.MethodName)
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
			return fmt.Errorf("mvc model attribute method %s.%s requires mvc controller receiver type", attribute.ControllerType, attribute.MethodName)
		}
		controller.ModelAttributes = append(controller.ModelAttributes, attribute)
	}
	for _, handler := range model.pendingExceptionHandlers {
		advice := model.adviceByType[handler.AdviceType]
		if advice == nil {
			return fmt.Errorf("mvc exception handler method %s.%s requires mvc controller advice receiver type", handler.AdviceType, handler.MethodName)
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
	if hasMVCRouteMappingAnnotation(item.Annotations()) || !hasAnnotation(item.Annotations(), "model-attribute") {
		return nil
	}
	attribute, err := buildMVCModelAttributeMethod(item.FileSet(), item.File(), item.FuncDecl(), item.Annotations())
	if err != nil {
		return err
	}
	attribute.ControllerType = item.ReceiverTypeName()
	attribute.MethodName = item.FuncName()
	model := ensureMVCAnnotationModel(ctx)
	model.pendingModelAttributes = append(model.pendingModelAttributes, attribute)
	return nil
}

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
