package generate

import (
	"fmt"
	"go/ast"
	"go/token"
)

func isMVCCrossOriginAnnotation(name string) bool { return name == "cross-origin" }

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

func mvcAnnotationExtension() AnnotationExtension {
	return AnnotationExtension{
		Name:        "mvc",
		Descriptors: mvcAnnotationDescriptors(),
		Binder:      mvcAnnotationBinder{},
		Generator:   mvcAnnotationGenerator{},
	}
}

func mvcAnnotationDescriptors() []AnnotationDescriptor {
	return []AnnotationDescriptor{
		typeDesc("controller", validateMVCControllerAnnotation),
		typeDesc("rest-controller", validateMVCControllerAnnotation),
		typeDesc("mvc-controller", validateMVCControllerAnnotation),
		typeDesc("controller-advice", validateMVCControllerAdviceAnnotation),
		typeDesc("rest-controller-advice", validateMVCControllerAdviceAnnotation),
		typeMethodDesc("request-mapping", validateMVCRequestMappingAnnotation),
		typeMethodDesc("cross-origin", validateMVCCrossOriginAnnotation),
		methodDesc("get", validateMVCHTTPMappingAnnotation),
		methodDesc("head", validateMVCHTTPMappingAnnotation),
		methodDesc("post", validateMVCHTTPMappingAnnotation),
		methodDesc("put", validateMVCHTTPMappingAnnotation),
		methodDesc("patch", validateMVCHTTPMappingAnnotation),
		methodDesc("delete", validateMVCHTTPMappingAnnotation),
		methodDesc("options", validateMVCHTTPMappingAnnotation),
		methodDesc("trace", validateMVCHTTPMappingAnnotation),
		methodDesc("request-body", validateMVCRequestBodyAnnotation),
		methodDesc("body", validateMVCRequestBodyAnnotation),
		methodDesc("request-entity", validateMVCRequestEntityAnnotation),
		methodDesc("multipart-body", validateMVCMultipartBodyAnnotation),
		methodDesc("path-variable", validateMVCParameterBindingAnnotation),
		methodDesc("request-param", validateMVCParameterBindingAnnotation),
		methodDesc("request-header", validateMVCParameterBindingAnnotation),
		methodDesc("cookie-value", validateMVCParameterBindingAnnotation),
		methodDesc("model-attribute", validateMVCModelAttributeAnnotation),
		methodDesc("request-attribute", validateMVCParameterBindingAnnotation),
		methodDesc("session-attribute", validateMVCParameterBindingAnnotation),
		methodDesc("matrix-variable", validateMVCParameterBindingAnnotation),
		methodDesc("request-part", validateMVCRequestPartAnnotation),
		methodDesc("validated", validateMVCValidatedAnnotation),
		methodDesc("response-body", validateMVCResponseBodyAnnotation),
		methodDesc("response-status", validateMVCResponseStatusAnnotation),
		methodDesc("exception-handler", validateMVCExceptionHandlerAnnotation),
	}
}

func validateMVCControllerAnnotation(ctx AnnotationValidationContext) error {
	typeSpec := ctx.Item.TypeSpec()
	if typeSpec == nil {
		return annotationError("requires type target", ctx.Annotation.Name)
	}
	if _, ok := typeSpec.Type.(*ast.StructType); !ok {
		return annotationError("requires struct type target", ctx.Annotation.Name)
	}
	if hasMVCControllerAdviceAnnotation(ctx.Item.Annotations()) {
		return annotationError("target must not also declare mvc controller advice", ctx.Annotation.Name)
	}
	return validateCoreNameAnnotation(ctx.Annotation)
}

func validateMVCRequestMappingAnnotation(ctx AnnotationValidationContext) error {
	if ctx.Target == AnnotationTargetType {
		if !hasMVCControllerAnnotation(ctx.Item.Annotations()) {
			return annotationError("on type requires mvc controller target", ctx.Annotation.Name)
		}
		if err := requireMVCPath(ctx.Annotation); err != nil {
			return err
		}
		_, err := mvcTypeRequestMethodsFromAnnotation(ctx.Annotation)
		return err
	}
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if _, err := mvcRouteMapping(ctx.Annotation); err != nil {
		return err
	}
	return nil
}

func validateMVCHTTPMappingAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	return requireMVCPath(ctx.Annotation)
}

func validateMVCRequestBodyAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return annotationError("requires mvc route method target", ctx.Annotation.Name)
	}
	selector := mvcRequestBodySelector(ctx.Annotation)
	if selector == "" {
		return annotationError("requires parameter selector", ctx.Annotation.Name)
	}
	if !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return annotationError("selector %q does not match any method parameter", ctx.Annotation.Name, selector)
	}
	return nil
}

func validateMVCMultipartBodyAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return annotationError("requires mvc route method target", ctx.Annotation.Name)
	}
	selector := mvcMultipartBodySelector(ctx.Annotation)
	if selector == "" {
		return annotationError("requires parameter selector", ctx.Annotation.Name)
	}
	if !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return annotationError("selector %q does not match any method parameter", ctx.Annotation.Name, selector)
	}
	return nil
}

func validateMVCParameterBindingAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return annotationError("requires mvc route method target", ctx.Annotation.Name)
	}
	selector := mvcBindingSelector(ctx.Annotation)
	if selector == "" {
		return annotationError("requires parameter selector", ctx.Annotation.Name)
	}
	if !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return annotationError("selector %q does not match any method parameter", ctx.Annotation.Name, selector)
	}
	if err := validateAtMostOneAnnotationValue(ctx.Annotation); err != nil {
		return err
	}
	if _, hasName := ctx.Annotation.Args["name"]; hasName {
		if _, hasValue := ctx.Annotation.Args["value"]; hasValue {
			return annotationError("accepts either name or value argument", ctx.Annotation.Name)
		}
	}
	if err := validateBoolArg(ctx.Annotation, "required"); err != nil {
		return err
	}
	return nil
}

func validateMVCModelAttributeAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return validateMVCModelAttributeMethodAnnotation(ctx)
	}
	selector := mvcBindingSelector(ctx.Annotation)
	if selector == "" {
		return annotationError("requires parameter selector", ctx.Annotation.Name)
	}
	if !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return annotationError("selector %q does not match any method parameter", ctx.Annotation.Name, selector)
	}
	return validateAtMostOneAnnotationValue(ctx.Annotation)
}

func validateMVCModelAttributeMethodAnnotation(ctx AnnotationValidationContext) error {
	if mvcModelAttributeAnnotationCount(ctx.Item.Annotations()) > 1 {
		return fmt.Errorf("mvc model attribute method %s has multiple model-attribute annotations", ctx.Item.FuncName())
	}
	if selector := normalizeSelector(ctx.Annotation.Selector); selector != "" {
		return annotationError("selector %q requires mvc route method target", ctx.Annotation.Name, selector)
	}
	if _, hasParam := ctx.Annotation.Args["param"]; hasParam {
		return annotationError("param argument requires mvc route method target", ctx.Annotation.Name)
	}
	if err := validateAtMostOneAnnotationValue(ctx.Annotation); err != nil {
		return err
	}
	if _, hasName := ctx.Annotation.Args["name"]; hasName {
		if _, hasValue := ctx.Annotation.Args["value"]; hasValue {
			return annotationError("accepts either name or value argument", ctx.Annotation.Name)
		}
	}
	name := mvcModelAttributeMethodName(ctx.Annotation)
	if name == "" {
		return annotationError("requires model attribute name", ctx.Annotation.Name)
	}
	fn := ctx.Item.FuncDecl()
	if fn == nil {
		return annotationError("requires concrete method with receiver", ctx.Annotation.Name)
	}
	if _, err := mvcModelAttributeMethodParams(ctx.Item.File(), fn); err != nil {
		return err
	}
	returnKind, err := mvcMethodReturnKind(ctx.Item.File(), fn)
	if err != nil {
		return err
	}
	if !mvcModelAttributeMethodSupportsReturn(returnKind) {
		return fmt.Errorf("mvc model attribute method %s must return T or T,error", fn.Name.Name)
	}
	return nil
}

func validateMVCRequestPartAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCParameterBindingAnnotation(ctx); err != nil {
		return err
	}
	if _, ok := ctx.Annotation.Args["defaultValue"]; ok {
		return annotationError("does not accept defaultValue argument", ctx.Annotation.Name)
	}
	if _, ok := ctx.Annotation.Args["default"]; ok {
		return annotationError("does not accept default argument", ctx.Annotation.Name)
	}
	return nil
}

func validateMVCValidatedAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return annotationError("requires mvc route method target", ctx.Annotation.Name)
	}
	if normalizeSelector(ctx.Annotation.Selector) != "" {
		return annotationError("does not accept selector", ctx.Annotation.Name)
	}
	if len(mvcValidationGroups(ctx.Annotation)) == 0 {
		return annotationError("requires validation group value", ctx.Annotation.Name)
	}
	return nil
}

func validateMVCResponseStatusAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) && !hasMVCExceptionHandlerAnnotation(ctx.Item.Annotations()) {
		return annotationError("requires mvc route or exception handler method target", ctx.Annotation.Name)
	}
	_, err := mvcResponseStatus(ctx.Annotation)
	return err
}

func validateMVCResponseBodyAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) && !hasMVCExceptionHandlerAnnotation(ctx.Item.Annotations()) {
		return annotationError("requires mvc route or exception handler method target", ctx.Annotation.Name)
	}
	if normalizeSelector(ctx.Annotation.Selector) != "" || len(ctx.Annotation.Args) > 0 || len(ctx.Annotation.Values) > 0 {
		return annotationError("does not accept arguments", ctx.Annotation.Name)
	}
	return nil
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

func analyzeMVCHandler(fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, annotations []Annotation) (mvcHandler, error) {
	if fn == nil {
		return mvcHandler{}, fmt.Errorf("mvc handler method is nil")
	}
	params, err := mvcMethodParams(fset, file, fn, annotations)
	if err != nil {
		return mvcHandler{}, err
	}
	returnKind, err := mvcMethodReturnKind(file, fn)
	if err != nil {
		return mvcHandler{}, err
	}
	if hasMVCBodyParam(params) && !mvcReturnSupportsRequestBody(returnKind) {
		return mvcHandler{}, mvcHandlerError("with request body must return T, T,error, web.ResponseEntity, or web.ResponseEntity,error", fn.Name.Name)
	}
	if hasMVCRequestEntityParam(params) && !mvcReturnSupportsRequestBody(returnKind) {
		return mvcHandler{}, mvcHandlerError("with request entity must return T, T,error, web.ResponseEntity, or web.ResponseEntity,error", fn.Name.Name)
	}
	if hasMVCMultipartBodyParam(params) && !mvcReturnSupportsRequestBody(returnKind) {
		return mvcHandler{}, mvcHandlerError("with multipart body must return T, T,error, web.ResponseEntity, or web.ResponseEntity,error", fn.Name.Name)
	}
	if hasMVCResponseBodyAnnotation(annotations) && hasMVCModelParam(params) {
		return mvcHandler{}, mvcHandlerError("response-body must not be used with *mvc.Model", fn.Name.Name)
	}
	if hasMVCValidatedAnnotation(annotations) && !hasMVCBodyParam(params) && !hasMVCRequestEntityParam(params) && !hasMVCMultipartBodyParam(params) && !hasMVCModelAttributeParam(params) && !hasMVCJSONRequestPartParam(params) {
		return mvcHandler{}, mvcHandlerError("validated requires request body, request entity, multipart body, model attribute, or JSON request part parameter", fn.Name.Name)
	}
	return mvcHandler{
		Params:     params,
		ReturnKind: returnKind,
		ReturnType: mvcPrimaryReturnType(fset, fn),
		EntityBody: mvcPrimaryResponseEntityBodyType(fset, file, fn),
	}, nil
}

func mvcReturnSupportsRequestBody(kind mvcReturnKind) bool {
	switch kind {
	case mvcReturnValue, mvcReturnValueError, mvcReturnEntity, mvcReturnEntityError:
		return true
	default:
		return false
	}
}
