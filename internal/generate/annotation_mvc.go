package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const (
	mvcAnnotationModelKey      = "goark.mvc.annotations"
	arkartaMultipartImportPath = "goark.dev/arkarta/servlet/multipart"
	arkartaWebImportPath       = "goark.dev/arkarta/web"
	goarkWebImportPath         = "goark.dev/goark/web"
	goarkWebCORSImportPath     = "goark.dev/goark/web/cors"
	goarkMVCImportPath         = "goark.dev/goark/web/mvc"
)

var defaultMVCRequestMappingMethods = [...]string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodOptions,
}

type mvcAnnotationModel struct {
	Controllers              []*mvcController
	Advices                  []*mvcControllerAdvice
	byType                   map[string]*mvcController
	adviceByType             map[string]*mvcControllerAdvice
	pending                  []mvcRoute
	pendingExceptionHandlers []mvcExceptionHandler
}

type mvcController struct {
	Component annotationComponent
	BasePaths []string
	Routes    []mvcRoute
	Kind      string

	Conditions  mvcRouteConditions
	CrossOrigin *mvcCrossOrigin
}

type mvcRoute struct {
	ControllerType   string
	MethodName       string
	HTTPMethod       string
	HTTPMethods      []string
	Path             string
	Paths            []string
	Status           int
	StatusExplicit   bool
	ResponseBody     bool
	ValidationGroups []string
	ControllerKind   string
	Conditions       mvcRouteConditions
	CrossOrigin      *mvcCrossOrigin
	Handler          mvcHandler
}

type mvcHandler struct {
	Params     []mvcHandlerParam
	ReturnKind mvcReturnKind
	ReturnType string
	EntityBody string
}

type mvcHandlerParam struct {
	Name            string
	Type            string
	BodyType        string
	Kind            mvcHandlerParamKind
	Binding         mvcParamBinding
	RequestPartFile bool
}

type mvcParamBinding struct {
	SourceName     string
	SourceExplicit bool
	Required       bool
	HasDefault     bool
	DefaultValue   string
}

type mvcHandlerParamKind uint8

const (
	mvcParamContext mvcHandlerParamKind = iota + 1
	mvcParamBody
	mvcParamRequestEntity
	mvcParamMultipartBody
	mvcParamPathVariable
	mvcParamRequestParam
	mvcParamRequestHeader
	mvcParamCookieValue
	mvcParamModelAttribute
	mvcParamRequestAttribute
	mvcParamSessionAttribute
	mvcParamMatrixVariable
	mvcParamRequestPart
	mvcParamModel
)

type mvcReturnKind uint8

const (
	mvcReturnNone mvcReturnKind = iota
	mvcReturnError
	mvcReturnResult
	mvcReturnResultError
	mvcReturnEntity
	mvcReturnEntityError
	mvcReturnValue
	mvcReturnValueError
)

type mvcAnnotationBinder struct{}

type mvcAnnotationGenerator struct{}

func mvcAnnotationExtension() AnnotationExtension {
	return AnnotationExtension{
		Descriptors: mvcAnnotationDescriptors(),
		Binder:      mvcAnnotationBinder{},
		Generator:   mvcAnnotationGenerator{},
	}
}

func mvcAnnotationDescriptors() []AnnotationDescriptor {
	return []AnnotationDescriptor{
		{Name: "controller", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateMVCControllerAnnotation},
		{Name: "rest-controller", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateMVCControllerAnnotation},
		{Name: "mvc-controller", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateMVCControllerAnnotation},
		{Name: "controller-advice", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateMVCControllerAdviceAnnotation},
		{Name: "rest-controller-advice", Targets: []AnnotationTarget{AnnotationTargetType}, Validate: validateMVCControllerAdviceAnnotation},
		{Name: "request-mapping", Targets: []AnnotationTarget{AnnotationTargetType, AnnotationTargetMethod}, Validate: validateMVCRequestMappingAnnotation},
		{Name: "cross-origin", Targets: []AnnotationTarget{AnnotationTargetType, AnnotationTargetMethod}, Validate: validateMVCCrossOriginAnnotation},
		{Name: "get", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "head", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "post", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "put", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "patch", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "delete", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "options", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "trace", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "request-body", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCRequestBodyAnnotation},
		{Name: "body", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCRequestBodyAnnotation},
		{Name: "request-entity", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCRequestEntityAnnotation},
		{Name: "multipart-body", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCMultipartBodyAnnotation},
		{Name: "path-variable", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCParameterBindingAnnotation},
		{Name: "request-param", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCParameterBindingAnnotation},
		{Name: "request-header", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCParameterBindingAnnotation},
		{Name: "cookie-value", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCParameterBindingAnnotation},
		{Name: "model-attribute", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCModelAttributeAnnotation},
		{Name: "request-attribute", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCParameterBindingAnnotation},
		{Name: "session-attribute", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCParameterBindingAnnotation},
		{Name: "matrix-variable", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCParameterBindingAnnotation},
		{Name: "request-part", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCRequestPartAnnotation},
		{Name: "validated", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCValidatedAnnotation},
		{Name: "response-body", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCResponseBodyAnnotation},
		{Name: "response-status", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCResponseStatusAnnotation},
		{Name: "exception-handler", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCExceptionHandlerAnnotation},
	}
}

func validateMVCControllerAnnotation(ctx AnnotationValidationContext) error {
	typeSpec := ctx.Item.TypeSpec()
	if typeSpec == nil {
		return fmt.Errorf("annotation %q requires type target", ctx.Annotation.Name)
	}
	if _, ok := typeSpec.Type.(*ast.StructType); !ok {
		return fmt.Errorf("annotation %q requires struct type target", ctx.Annotation.Name)
	}
	if hasMVCControllerAdviceAnnotation(ctx.Item.Annotations()) {
		return fmt.Errorf("annotation %q target must not also declare mvc controller advice", ctx.Annotation.Name)
	}
	return validateCoreNameAnnotation(ctx.Annotation)
}

func validateMVCRequestMappingAnnotation(ctx AnnotationValidationContext) error {
	if ctx.Target == AnnotationTargetType {
		if !hasMVCControllerAnnotation(ctx.Item.Annotations()) {
			return fmt.Errorf("annotation %q on type requires mvc controller target", ctx.Annotation.Name)
		}
		return requireMVCPath(ctx.Annotation)
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
		return fmt.Errorf("annotation %q requires mvc route method target", ctx.Annotation.Name)
	}
	selector := mvcRequestBodySelector(ctx.Annotation)
	if selector == "" {
		return fmt.Errorf("annotation %q requires parameter selector", ctx.Annotation.Name)
	}
	if !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return fmt.Errorf("annotation %q selector %q does not match any method parameter", ctx.Annotation.Name, selector)
	}
	return nil
}

func validateMVCMultipartBodyAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return fmt.Errorf("annotation %q requires mvc route method target", ctx.Annotation.Name)
	}
	selector := mvcMultipartBodySelector(ctx.Annotation)
	if selector == "" {
		return fmt.Errorf("annotation %q requires parameter selector", ctx.Annotation.Name)
	}
	if !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return fmt.Errorf("annotation %q selector %q does not match any method parameter", ctx.Annotation.Name, selector)
	}
	return nil
}

func validateMVCParameterBindingAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return fmt.Errorf("annotation %q requires mvc route method target", ctx.Annotation.Name)
	}
	selector := mvcBindingSelector(ctx.Annotation)
	if selector == "" {
		return fmt.Errorf("annotation %q requires parameter selector", ctx.Annotation.Name)
	}
	if !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return fmt.Errorf("annotation %q selector %q does not match any method parameter", ctx.Annotation.Name, selector)
	}
	if err := validateAtMostOneAnnotationValue(ctx.Annotation); err != nil {
		return err
	}
	if _, hasName := ctx.Annotation.Args["name"]; hasName {
		if _, hasValue := ctx.Annotation.Args["value"]; hasValue {
			return fmt.Errorf("annotation %q accepts either name or value argument", ctx.Annotation.Name)
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
		return fmt.Errorf("annotation %q requires mvc route method target", ctx.Annotation.Name)
	}
	selector := mvcBindingSelector(ctx.Annotation)
	if selector == "" {
		return fmt.Errorf("annotation %q requires parameter selector", ctx.Annotation.Name)
	}
	if !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return fmt.Errorf("annotation %q selector %q does not match any method parameter", ctx.Annotation.Name, selector)
	}
	return validateAtMostOneAnnotationValue(ctx.Annotation)
}

func validateMVCRequestPartAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCParameterBindingAnnotation(ctx); err != nil {
		return err
	}
	if _, ok := ctx.Annotation.Args["defaultValue"]; ok {
		return fmt.Errorf("annotation %q does not accept defaultValue argument", ctx.Annotation.Name)
	}
	if _, ok := ctx.Annotation.Args["default"]; ok {
		return fmt.Errorf("annotation %q does not accept default argument", ctx.Annotation.Name)
	}
	return nil
}

func validateMVCValidatedAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return fmt.Errorf("annotation %q requires mvc route method target", ctx.Annotation.Name)
	}
	if normalizeSelector(ctx.Annotation.Selector) != "" {
		return fmt.Errorf("annotation %q does not accept selector", ctx.Annotation.Name)
	}
	if len(mvcValidationGroups(ctx.Annotation)) == 0 {
		return fmt.Errorf("annotation %q requires validation group value", ctx.Annotation.Name)
	}
	return nil
}

func validateMVCResponseStatusAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) && !hasMVCExceptionHandlerAnnotation(ctx.Item.Annotations()) {
		return fmt.Errorf("annotation %q requires mvc route or exception handler method target", ctx.Annotation.Name)
	}
	_, err := mvcResponseStatus(ctx.Annotation)
	return err
}

func validateMVCResponseBodyAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if !hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) && !hasMVCExceptionHandlerAnnotation(ctx.Item.Annotations()) {
		return fmt.Errorf("annotation %q requires mvc route or exception handler method target", ctx.Annotation.Name)
	}
	if normalizeSelector(ctx.Annotation.Selector) != "" || len(ctx.Annotation.Args) > 0 || len(ctx.Annotation.Values) > 0 {
		return fmt.Errorf("annotation %q does not accept arguments", ctx.Annotation.Name)
	}
	return nil
}

func validateMVCHandlerMethod(ctx AnnotationValidationContext) error {
	fn := ctx.Item.FuncDecl()
	if fn == nil || fn.Recv == nil {
		return fmt.Errorf("annotation %q requires concrete method with receiver", ctx.Annotation.Name)
	}
	if ctx.Item.ReceiverTypeName() == "" {
		return fmt.Errorf("annotation %q receiver is not supported", ctx.Annotation.Name)
	}
	return nil
}

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
		controller.Routes = append(controller.Routes, expandMVCRoutePaths(controller, route)...)
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

func buildMVCController(fset *token.FileSet, typeSpec *ast.TypeSpec, annotations []Annotation) (*mvcController, error) {
	component, err := buildMVCComponent(fset, typeSpec, annotations, mvcControllerKind(annotations))
	if err != nil {
		return nil, err
	}
	crossOrigin, err := mvcCrossOriginFromAnnotations(annotations)
	if err != nil {
		return nil, err
	}
	return &mvcController{
		Component:   component,
		BasePaths:   mvcTypeBasePaths(annotations),
		Kind:        mvcControllerKind(annotations),
		Conditions:  mvcTypeRouteConditions(annotations),
		CrossOrigin: crossOrigin,
	}, nil
}

func buildMVCComponent(fset *token.FileSet, typeSpec *ast.TypeSpec, annotations []Annotation, kind string) (annotationComponent, error) {
	typeName := typeSpec.Name.Name
	component := annotationComponent{
		TypeName: typeName,
		Name:     annotationName(annotations, kind, lowerCamel(typeName)),
	}
	structType, _ := typeSpec.Type.(*ast.StructType)
	if structType == nil {
		return component, nil
	}
	for _, field := range structType.Fields.List {
		fieldAnnotations, err := parseAnnotations(field.Doc)
		if err != nil {
			return annotationComponent{}, err
		}
		if len(field.Names) == 0 {
			continue
		}
		for _, name := range field.Names {
			injection := buildInjection(fieldAnnotations, name.Name)
			if injection.Kind == "" {
				continue
			}
			component.Fields = append(component.Fields, annotationField{
				Name:      name.Name,
				Type:      exprString(fset, field.Type),
				Injection: injection,
			})
		}
	}
	return component, nil
}

func buildMVCRoute(fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, annotations []Annotation) (mvcRoute, error) {
	mapping, err := mvcRouteFromAnnotations(annotations)
	if err != nil {
		return mvcRoute{}, err
	}
	handler, err := analyzeMVCHandler(fset, file, fn, annotations)
	if err != nil {
		return mvcRoute{}, err
	}
	return mvcRoute{
		HTTPMethod:       mapping.methods[0],
		HTTPMethods:      mapping.methods,
		Path:             mapping.paths[0],
		Paths:            mapping.paths,
		Status:           mapping.status,
		StatusExplicit:   mapping.explicitStatus,
		ResponseBody:     mapping.responseBody,
		ValidationGroups: mapping.validationGroups,
		Conditions:       mapping.conditions,
		CrossOrigin:      mapping.crossOrigin,
		Handler:          handler,
	}, nil
}

type mvcRouteMappingSpec struct {
	methods          []string
	paths            []string
	status           int
	explicitStatus   bool
	responseBody     bool
	validationGroups []string
	conditions       mvcRouteConditions
	crossOrigin      *mvcCrossOrigin
}

func mvcRouteFromAnnotations(annotations []Annotation) (mvcRouteMappingSpec, error) {
	var out mvcRouteMappingSpec
	responseStatus := 0
	hasResponseStatus := false
	hasResponseBody := false
	hasValidated := false
	hasMapping := false
	for _, annotation := range annotations {
		if isMVCValidatedAnnotation(annotation.Name) {
			if hasValidated {
				return mvcRouteMappingSpec{}, fmt.Errorf("mvc route method has multiple validated annotations")
			}
			out.validationGroups = mvcValidationGroups(annotation)
			hasValidated = true
			continue
		}
		if isMVCResponseBodyAnnotation(annotation.Name) {
			if hasResponseBody {
				return mvcRouteMappingSpec{}, fmt.Errorf("mvc route method has multiple response-body annotations")
			}
			out.responseBody = true
			hasResponseBody = true
			continue
		}
		if isMVCResponseStatusAnnotation(annotation.Name) {
			if hasResponseStatus {
				return mvcRouteMappingSpec{}, fmt.Errorf("mvc route method has multiple response-status annotations")
			}
			status, err := mvcResponseStatus(annotation)
			if err != nil {
				return mvcRouteMappingSpec{}, err
			}
			responseStatus = status
			hasResponseStatus = true
			continue
		}
		if isMVCRouteMappingAnnotation(annotation.Name) {
			if hasMapping {
				return mvcRouteMappingSpec{}, fmt.Errorf("mvc route method has multiple mapping annotations")
			}
			mapping, err := mvcRouteMapping(annotation)
			if err != nil {
				return mvcRouteMappingSpec{}, err
			}
			out = mapping
			hasMapping = true
		}
	}
	crossOrigin, err := mvcCrossOriginFromAnnotations(annotations)
	if err != nil {
		return mvcRouteMappingSpec{}, err
	}
	out.crossOrigin = crossOrigin
	if !hasMapping {
		return mvcRouteMappingSpec{}, fmt.Errorf("mvc route method requires mapping annotation")
	}
	if hasResponseStatus {
		if out.explicitStatus {
			return mvcRouteMappingSpec{}, fmt.Errorf("mvc route method must not declare both mapping status and response-status")
		}
		out.status = responseStatus
		out.explicitStatus = true
	}
	return out, nil
}

func mvcRouteMapping(annotation Annotation) (mvcRouteMappingSpec, error) {
	paths, err := requireMVCPathTexts(annotation)
	if err != nil {
		return mvcRouteMappingSpec{}, err
	}
	methods, err := mvcHTTPMethods(annotation)
	if err != nil {
		return mvcRouteMappingSpec{}, err
	}
	explicitStatus := mvcMappingHasExplicitStatus(annotation)
	status, err := mvcStatus(annotation, defaultMVCStatus(methods))
	if err != nil {
		return mvcRouteMappingSpec{}, err
	}
	return mvcRouteMappingSpec{
		methods:        methods,
		paths:          normalizeMVCPaths(paths),
		status:         status,
		explicitStatus: explicitStatus,
		conditions:     mvcRouteConditionsFromAnnotation(annotation),
	}, nil
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
		return mvcHandler{}, fmt.Errorf("mvc handler method %s with request body must return T, T,error, web.ResponseEntity, or web.ResponseEntity,error", fn.Name.Name)
	}
	if hasMVCRequestEntityParam(params) && !mvcReturnSupportsRequestBody(returnKind) {
		return mvcHandler{}, fmt.Errorf("mvc handler method %s with request entity must return T, T,error, web.ResponseEntity, or web.ResponseEntity,error", fn.Name.Name)
	}
	if hasMVCMultipartBodyParam(params) && !mvcReturnSupportsRequestBody(returnKind) {
		return mvcHandler{}, fmt.Errorf("mvc handler method %s with multipart body must return T, T,error, web.ResponseEntity, or web.ResponseEntity,error", fn.Name.Name)
	}
	if hasMVCResponseBodyAnnotation(annotations) && hasMVCModelParam(params) {
		return mvcHandler{}, fmt.Errorf("mvc handler method %s response-body must not be used with *mvc.Model", fn.Name.Name)
	}
	if hasMVCValidatedAnnotation(annotations) && !hasMVCBodyParam(params) && !hasMVCRequestEntityParam(params) && !hasMVCMultipartBodyParam(params) && !hasMVCModelAttributeParam(params) && !hasMVCJSONRequestPartParam(params) {
		return mvcHandler{}, fmt.Errorf("mvc handler method %s validated requires request body, request entity, multipart body, model attribute, or JSON request part parameter", fn.Name.Name)
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

func mvcMethodParams(fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, annotations []Annotation) ([]mvcHandlerParam, error) {
	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		if len(mvcRequestBodySelectors(annotations)) > 0 {
			return nil, fmt.Errorf("mvc handler method %s request body selector does not match any method parameter", fn.Name.Name)
		}
		if len(mvcRequestEntitySelectors(annotations)) > 0 {
			return nil, fmt.Errorf("mvc handler method %s request entity selector does not match any method parameter", fn.Name.Name)
		}
		if len(mvcMultipartBodySelectors(annotations)) > 0 {
			return nil, fmt.Errorf("mvc handler method %s multipart body selector does not match any method parameter", fn.Name.Name)
		}
		return nil, nil
	}

	bodySelectors := mvcRequestBodySelectorSet(annotations)
	requestEntitySelectors := mvcRequestEntitySelectorSet(annotations)
	multipartBodySelectors := mvcMultipartBodySelectorSet(annotations)
	paramBindings, err := mvcParameterBindingSet(annotations)
	if err != nil {
		return nil, err
	}
	params := make([]mvcHandlerParam, 0, len(fn.Type.Params.List))
	contextSeen := false
	bodySeen := false
	modelSeen := false
	for index, field := range fn.Type.Params.List {
		names := field.Names
		if len(names) == 0 {
			names = []*ast.Ident{ast.NewIdent(fmt.Sprintf("arg%d", index))}
		}
		if len(names) != 1 {
			return nil, fmt.Errorf("mvc handler method %s parameter group must declare exactly one name", fn.Name.Name)
		}
		name := names[0].Name
		if isArkWebContextExpr(file, field.Type) {
			if _, isBody := bodySelectors[name]; isBody {
				return nil, fmt.Errorf("mvc handler method %s request body parameter %s must not be *arkarta/web.Context", fn.Name.Name, name)
			}
			if _, isRequestEntity := requestEntitySelectors[name]; isRequestEntity {
				return nil, fmt.Errorf("mvc handler method %s request entity parameter %s must not be *arkarta/web.Context", fn.Name.Name, name)
			}
			if _, isMultipartBody := multipartBodySelectors[name]; isMultipartBody {
				return nil, fmt.Errorf("mvc handler method %s multipart body parameter %s must not be *arkarta/web.Context", fn.Name.Name, name)
			}
			if _, isBound := paramBindings[name]; isBound {
				return nil, fmt.Errorf("mvc handler method %s bound parameter %s must not be *arkarta/web.Context", fn.Name.Name, name)
			}
			if contextSeen {
				return nil, fmt.Errorf("mvc handler method %s must not declare multiple *arkarta/web.Context parameters", fn.Name.Name)
			}
			contextSeen = true
			params = append(params, mvcHandlerParam{Name: name, Kind: mvcParamContext})
			continue
		}
		if isGoarkMVCModelPointerExpr(file, field.Type) {
			if _, isBody := bodySelectors[name]; isBody {
				return nil, fmt.Errorf("mvc handler method %s request body parameter %s must not be *mvc.Model", fn.Name.Name, name)
			}
			if _, isRequestEntity := requestEntitySelectors[name]; isRequestEntity {
				return nil, fmt.Errorf("mvc handler method %s request entity parameter %s must not be *mvc.Model", fn.Name.Name, name)
			}
			if _, isMultipartBody := multipartBodySelectors[name]; isMultipartBody {
				return nil, fmt.Errorf("mvc handler method %s multipart body parameter %s must not be *mvc.Model", fn.Name.Name, name)
			}
			if _, isBound := paramBindings[name]; isBound {
				return nil, fmt.Errorf("mvc handler method %s bound parameter %s must not be *mvc.Model", fn.Name.Name, name)
			}
			if modelSeen {
				return nil, fmt.Errorf("mvc handler method %s must not declare multiple *mvc.Model parameters", fn.Name.Name)
			}
			modelSeen = true
			params = append(params, mvcHandlerParam{Name: name, Type: "mvc.Model", Kind: mvcParamModel})
			continue
		}
		if isSelectorTypeExpr(field.Type, "Context") {
			return nil, fmt.Errorf("mvc handler method %s parameter must be *arkarta/web.Context", fn.Name.Name)
		}
		requestEntityBodyType, isRequestEntityType := mvcRequestEntityBodyType(fset, file, field.Type)
		if _, isRequestEntity := requestEntitySelectors[name]; isRequestEntity || isRequestEntityType {
			if _, isBody := bodySelectors[name]; isBody {
				return nil, fmt.Errorf("mvc handler method %s parameter %s must not declare multiple mvc binding annotations", fn.Name.Name, name)
			}
			if _, isMultipartBody := multipartBodySelectors[name]; isMultipartBody {
				return nil, fmt.Errorf("mvc handler method %s parameter %s must not declare multiple mvc binding annotations", fn.Name.Name, name)
			}
			if _, isBound := paramBindings[name]; isBound {
				return nil, fmt.Errorf("mvc handler method %s parameter %s must not declare multiple mvc binding annotations", fn.Name.Name, name)
			}
			if !isRequestEntityType {
				return nil, fmt.Errorf("mvc handler method %s request entity parameter %s must be goark.dev/goark/web.RequestEntity[T]", fn.Name.Name, name)
			}
			if bodySeen {
				return nil, fmt.Errorf("mvc handler method %s must not declare multiple request body parameters", fn.Name.Name)
			}
			bodySeen = true
			params = append(params, mvcHandlerParam{Name: name, Type: "goweb.RequestEntity[" + requestEntityBodyType + "]", Kind: mvcParamRequestEntity, BodyType: requestEntityBodyType})
			continue
		}
		if _, isBody := bodySelectors[name]; isBody {
			if _, isMultipartBody := multipartBodySelectors[name]; isMultipartBody {
				return nil, fmt.Errorf("mvc handler method %s parameter %s must not declare multiple mvc binding annotations", fn.Name.Name, name)
			}
			if _, isBound := paramBindings[name]; isBound {
				return nil, fmt.Errorf("mvc handler method %s parameter %s must not declare multiple mvc binding annotations", fn.Name.Name, name)
			}
			if bodySeen {
				return nil, fmt.Errorf("mvc handler method %s must not declare multiple request body parameters", fn.Name.Name)
			}
			bodySeen = true
			params = append(params, mvcHandlerParam{Name: name, Type: exprString(fset, field.Type), Kind: mvcParamBody})
			continue
		}
		if _, isMultipartBody := multipartBodySelectors[name]; isMultipartBody {
			if _, isBound := paramBindings[name]; isBound {
				return nil, fmt.Errorf("mvc handler method %s parameter %s must not declare multiple mvc binding annotations", fn.Name.Name, name)
			}
			if bodySeen {
				return nil, fmt.Errorf("mvc handler method %s must not declare multiple request body parameters", fn.Name.Name)
			}
			bodySeen = true
			params = append(params, mvcHandlerParam{Name: name, Type: exprString(fset, field.Type), Kind: mvcParamMultipartBody})
			continue
		}
		if binding, isBound := paramBindings[name]; isBound {
			typ := exprString(fset, field.Type)
			if binding.Kind == mvcParamModelAttribute && !isMVCModelAttributeTypeExpr(field.Type) {
				return nil, fmt.Errorf("mvc handler method %s model attribute parameter %s must be a non-pointer struct value", fn.Name.Name, name)
			}
			requestPartFile := binding.Kind == mvcParamRequestPart && isArkartaMultipartPartExpr(file, field.Type)
			if err := validateMVCParameterMapBinding(fn.Name.Name, name, typ, binding.Kind, binding.Binding); err != nil {
				return nil, err
			}
			if _, ok := mvcParameterBindingCall(mvcHandlerParam{Type: typ, Kind: binding.Kind, Binding: binding.Binding, RequestPartFile: requestPartFile}, nil); !ok {
				return nil, fmt.Errorf("mvc handler method %s parameter %s has unsupported mvc parameter type %s", fn.Name.Name, name, typ)
			}
			params = append(params, mvcHandlerParam{Name: name, Type: typ, Kind: binding.Kind, Binding: binding.Binding, RequestPartFile: requestPartFile})
			continue
		}
		return nil, fmt.Errorf("mvc handler method %s parameter %s must be *arkarta/web.Context or annotated with mvc binding annotation", fn.Name.Name, name)
	}
	for selector := range bodySelectors {
		if !mvcHasParam(params, selector) {
			return nil, fmt.Errorf("mvc handler method %s request body selector %q does not match any method parameter", fn.Name.Name, selector)
		}
	}
	for selector := range requestEntitySelectors {
		if !mvcHasParam(params, selector) {
			return nil, fmt.Errorf("mvc handler method %s request entity selector %q does not match any method parameter", fn.Name.Name, selector)
		}
	}
	for selector := range multipartBodySelectors {
		if !mvcHasParam(params, selector) {
			return nil, fmt.Errorf("mvc handler method %s multipart body selector %q does not match any method parameter", fn.Name.Name, selector)
		}
	}
	for selector := range paramBindings {
		if !mvcHasParam(params, selector) {
			return nil, fmt.Errorf("mvc handler method %s parameter binding selector %q does not match any method parameter", fn.Name.Name, selector)
		}
	}
	if bodySeen && hasMVCModelAttributeParam(params) {
		return nil, fmt.Errorf("mvc handler method %s must not combine request body and model attribute parameters", fn.Name.Name)
	}
	return params, nil
}

func mvcMethodReturnKind(file *ast.File, fn *ast.FuncDecl) (mvcReturnKind, error) {
	results := fn.Type.Results
	if results == nil || len(results.List) == 0 {
		return mvcReturnNone, nil
	}
	if len(results.List) == 1 {
		result := results.List[0].Type
		switch {
		case isErrorExpr(result):
			return mvcReturnError, nil
		case isArkWebResultExpr(file, result):
			return mvcReturnResult, nil
		case isGoarkWebDownloadResultExpr(file, result):
			return mvcReturnResult, nil
		case isGoarkWebResponseEntityExpr(file, result):
			return mvcReturnEntity, nil
		default:
			return mvcReturnValue, nil
		}
	}
	if len(results.List) == 2 && isErrorExpr(results.List[1].Type) {
		if isArkWebResultExpr(file, results.List[0].Type) {
			return mvcReturnResultError, nil
		}
		if isGoarkWebDownloadResultExpr(file, results.List[0].Type) {
			return mvcReturnResultError, nil
		}
		if isGoarkWebResponseEntityExpr(file, results.List[0].Type) {
			return mvcReturnEntityError, nil
		}
		return mvcReturnValueError, nil
	}
	return 0, fmt.Errorf("mvc handler method %s must return void, error, T, T,error, web.Result, web.Result,error, web.ResponseEntity, web.ResponseEntity,error, web.DownloadResult, or web.DownloadResult,error", fn.Name.Name)
}

func mvcPrimaryReturnType(fset *token.FileSet, fn *ast.FuncDecl) string {
	if fn == nil || fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return ""
	}
	return exprString(fset, fn.Type.Results.List[0].Type)
}

func mvcPrimaryResponseEntityBodyType(fset *token.FileSet, file *ast.File, fn *ast.FuncDecl) string {
	if fn == nil || fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return ""
	}
	body, _ := mvcResponseEntityBodyType(fset, file, fn.Type.Results.List[0].Type)
	return body
}

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

func writeMVCBindJSONHandler(builder *bytes.Buffer, route mvcRoute) {
	bodyParam, _ := mvcBodyParam(route.Handler.Params)
	if len(route.ValidationGroups) > 0 {
		builder.WriteString("mvc.BindJSONGroups[")
	} else {
		builder.WriteString("mvc.BindJSON[")
	}
	builder.WriteString(bodyParam.Type)
	builder.WriteString(", any](")
	builder.WriteString(strconv.Itoa(route.Status))
	builder.WriteString(", func(ctx *arkweb.Context, ")
	builder.WriteString(bodyParam.Name)
	builder.WriteByte(' ')
	builder.WriteString(bodyParam.Type)
	builder.WriteString(") (any, error) {\n")
	writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err", route.ValidationGroups)
	builder.WriteString("return ")
	builder.WriteString(mvcHandlerCall(route.MethodName, route.Handler.Params))
	if route.Handler.ReturnKind == mvcReturnValue {
		builder.WriteString(", nil")
	}
	builder.WriteString("\n}")
	writeMVCValidationGroupArguments(builder, route.ValidationGroups)
	builder.WriteByte(')')
}

func writeMVCBindEntityHandler(builder *bytes.Buffer, route mvcRoute) {
	if len(route.ValidationGroups) > 0 && route.Handler.EntityBody != "" {
		writeMVCBindEntityGroupsHandler(builder, route)
		return
	}
	bodyParam, _ := mvcBodyParam(route.Handler.Params)
	builder.WriteString("mvc.Handler(func(ctx *arkweb.Context) (arkweb.Result, error) {\n")
	builder.WriteString("var ")
	builder.WriteString(bodyParam.Name)
	builder.WriteByte(' ')
	builder.WriteString(bodyParam.Type)
	builder.WriteString("\nif err := ctx.BindAndValidateJSON(&")
	builder.WriteString(bodyParam.Name)
	builder.WriteString("); err != nil {\nreturn nil, err\n}\n")
	writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err", route.ValidationGroups)
	builder.WriteString("return ")
	builder.WriteString(mvcHandlerCall(route.MethodName, route.Handler.Params))
	if route.Handler.ReturnKind == mvcReturnEntity {
		builder.WriteString(", nil")
	}
	builder.WriteString("\n})")
}

func writeMVCBindMultipartHandler(builder *bytes.Buffer, route mvcRoute) {
	bodyParam, _ := mvcMultipartBodyParam(route.Handler.Params)
	if len(route.ValidationGroups) > 0 {
		builder.WriteString("mvc.BindMultipartGroups[")
	} else {
		builder.WriteString("mvc.BindMultipart[")
	}
	builder.WriteString(bodyParam.Type)
	builder.WriteString(", any](")
	builder.WriteString(strconv.Itoa(route.Status))
	builder.WriteString(", func(ctx *arkweb.Context, ")
	builder.WriteString(bodyParam.Name)
	builder.WriteByte(' ')
	builder.WriteString(bodyParam.Type)
	builder.WriteString(") (any, error) {\n")
	writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err", route.ValidationGroups)
	builder.WriteString("return ")
	builder.WriteString(mvcHandlerCall(route.MethodName, route.Handler.Params))
	if route.Handler.ReturnKind == mvcReturnValue {
		builder.WriteString(", nil")
	}
	builder.WriteString("\n}")
	if len(route.ValidationGroups) > 0 {
		builder.WriteString(", ")
		writeMVCValidationGroupSlice(builder, route.ValidationGroups)
	}
	builder.WriteByte(')')
}

func writeMVCBindMultipartEntityHandler(builder *bytes.Buffer, route mvcRoute) {
	if len(route.ValidationGroups) > 0 && route.Handler.EntityBody != "" {
		writeMVCBindMultipartEntityGroupsHandler(builder, route)
		return
	}
	bodyParam, _ := mvcMultipartBodyParam(route.Handler.Params)
	builder.WriteString("mvc.Handler(func(ctx *arkweb.Context) (arkweb.Result, error) {\n")
	builder.WriteString(bodyParam.Name)
	builder.WriteString(", err := mvc.Multipart[")
	builder.WriteString(bodyParam.Type)
	builder.WriteString("](ctx)\nif err != nil {\nreturn nil, err\n}\n")
	writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err", route.ValidationGroups)
	builder.WriteString("return ")
	builder.WriteString(mvcHandlerCall(route.MethodName, route.Handler.Params))
	if route.Handler.ReturnKind == mvcReturnEntity {
		builder.WriteString(", nil")
	}
	builder.WriteString("\n})")
}

func writeMVCBindEntityGroupsHandler(builder *bytes.Buffer, route mvcRoute) {
	bodyParam, _ := mvcBodyParam(route.Handler.Params)
	builder.WriteString("mvc.BindEntityGroups[")
	builder.WriteString(bodyParam.Type)
	builder.WriteString(", ")
	builder.WriteString(route.Handler.EntityBody)
	builder.WriteString("](func(ctx *arkweb.Context, ")
	builder.WriteString(bodyParam.Name)
	builder.WriteByte(' ')
	builder.WriteString(bodyParam.Type)
	builder.WriteString(") (goweb.ResponseEntity[")
	builder.WriteString(route.Handler.EntityBody)
	builder.WriteString("], error) {\n")
	writeMVCParameterBindings(builder, route.Handler.Params, "return goweb.ResponseEntity["+route.Handler.EntityBody+"]{}, err", route.ValidationGroups)
	builder.WriteString("return ")
	builder.WriteString(mvcHandlerCall(route.MethodName, route.Handler.Params))
	if route.Handler.ReturnKind == mvcReturnEntity {
		builder.WriteString(", nil")
	}
	builder.WriteString("\n}")
	writeMVCValidationGroupArguments(builder, route.ValidationGroups)
	builder.WriteByte(')')
}

func writeMVCBindMultipartEntityGroupsHandler(builder *bytes.Buffer, route mvcRoute) {
	bodyParam, _ := mvcMultipartBodyParam(route.Handler.Params)
	builder.WriteString("mvc.BindMultipartEntityGroups[")
	builder.WriteString(bodyParam.Type)
	builder.WriteString(", ")
	builder.WriteString(route.Handler.EntityBody)
	builder.WriteString("](func(ctx *arkweb.Context, ")
	builder.WriteString(bodyParam.Name)
	builder.WriteByte(' ')
	builder.WriteString(bodyParam.Type)
	builder.WriteString(") (goweb.ResponseEntity[")
	builder.WriteString(route.Handler.EntityBody)
	builder.WriteString("], error) {\n")
	writeMVCParameterBindings(builder, route.Handler.Params, "return goweb.ResponseEntity["+route.Handler.EntityBody+"]{}, err", route.ValidationGroups)
	builder.WriteString("return ")
	builder.WriteString(mvcHandlerCall(route.MethodName, route.Handler.Params))
	if route.Handler.ReturnKind == mvcReturnEntity {
		builder.WriteString(", nil")
	}
	builder.WriteString("\n}, ")
	writeMVCValidationGroupSlice(builder, route.ValidationGroups)
	builder.WriteByte(')')
}

func writeMVCValidationGroupArguments(builder *bytes.Buffer, groups []string) {
	for _, group := range groups {
		builder.WriteString(", ")
		builder.WriteString(strconv.Quote(group))
	}
}

func writeMVCValidationGroupSlice(builder *bytes.Buffer, groups []string) {
	builder.WriteString("[]string{")
	for index, group := range groups {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(strconv.Quote(group))
	}
	builder.WriteByte('}')
}

func mvcValidationGroupArguments(groups []string) string {
	if len(groups) == 0 {
		return ""
	}
	var builder bytes.Buffer
	writeMVCValidationGroupArguments(&builder, groups)
	return builder.String()
}

func mvcHandlerCall(methodName string, params []mvcHandlerParam) string {
	args := make([]string, 0, len(params))
	for _, param := range params {
		switch param.Kind {
		case mvcParamContext:
			args = append(args, "ctx")
		case mvcParamModel:
			args = append(args, "&"+param.Name)
		case mvcParamBody, mvcParamRequestEntity, mvcParamMultipartBody:
			args = append(args, param.Name)
		case mvcParamPathVariable, mvcParamRequestParam, mvcParamRequestHeader, mvcParamCookieValue, mvcParamModelAttribute,
			mvcParamRequestAttribute, mvcParamSessionAttribute, mvcParamMatrixVariable, mvcParamRequestPart:
			args = append(args, param.Name)
		}
	}
	return "controller." + methodName + "(" + strings.Join(args, ", ") + ")"
}

func writeMVCParameterBindings(builder *bytes.Buffer, params []mvcHandlerParam, errorReturn string, validationGroups []string) {
	for _, param := range params {
		if param.Kind == mvcParamModel {
			builder.WriteString(param.Name)
			builder.WriteString(" := mvc.NewModel()\n")
			continue
		}
		call, ok := mvcParameterBindingCall(param, validationGroups)
		if !ok {
			continue
		}
		builder.WriteString(param.Name)
		builder.WriteString(", err := ")
		builder.WriteString(call)
		builder.WriteString("\nif err != nil {\n")
		builder.WriteString(errorReturn)
		builder.WriteString("\n}\n")
	}
}

func mvcParameterBindingCall(param mvcHandlerParam, validationGroups []string) (string, bool) {
	if param.Kind == mvcParamModelAttribute {
		if len(validationGroups) > 0 {
			return "mvc.ModelAttributeGroups[" + param.Type + "](ctx" + mvcValidationGroupArguments(validationGroups) + ")", true
		}
		return "mvc.ModelAttribute[" + param.Type + "](ctx)", true
	}
	if param.Kind == mvcParamRequestPart {
		return mvcRequestPartBindingCall(param, validationGroups)
	}
	if function, ok := mvcParameterMapFunction(param.Kind, param.Type); ok {
		return "mvc." + function + "(ctx)", true
	}
	function, ok := mvcParameterFunction(param.Kind, param.Type)
	if !ok {
		return "", false
	}
	args := []string{"ctx", strconv.Quote(param.Binding.SourceName)}
	if param.Binding.HasDefault {
		args = append(args, "mvc.WithDefaultValue("+strconv.Quote(param.Binding.DefaultValue)+")")
	} else if !param.Binding.Required {
		args = append(args, "mvc.WithRequired(false)")
	}
	return "mvc." + function + "(" + strings.Join(args, ", ") + ")", true
}

func mvcParameterFunction(kind mvcHandlerParamKind, typ string) (string, bool) {
	suffix, ok := mvcParameterFunctionSuffix(kind, typ)
	if !ok {
		return "", false
	}
	switch kind {
	case mvcParamPathVariable:
		return "Path" + suffix, true
	case mvcParamRequestParam:
		return "RequestParam" + suffix, true
	case mvcParamRequestHeader:
		return "RequestHeader" + suffix, true
	case mvcParamCookieValue:
		return "CookieValue" + suffix, true
	case mvcParamModelAttribute:
		return "ModelAttribute", true
	case mvcParamRequestAttribute:
		return "RequestAttribute" + suffix, true
	case mvcParamSessionAttribute:
		return "SessionAttribute" + suffix, true
	case mvcParamMatrixVariable:
		return "MatrixVariable" + suffix, true
	default:
		return "", false
	}
}

func mvcParameterFunctionSuffix(kind mvcHandlerParamKind, typ string) (string, bool) {
	if kind == mvcParamRequestAttribute || kind == mvcParamSessionAttribute {
		return mvcScalarParameterTypeSuffix(typ)
	}
	return mvcCollectionParameterTypeSuffix(typ)
}

func mvcCollectionParameterTypeSuffix(typ string) (string, bool) {
	switch strings.TrimSpace(typ) {
	case "string":
		return "String", true
	case "int":
		return "Int", true
	case "int64":
		return "Int64", true
	case "bool":
		return "Bool", true
	case "float64":
		return "Float64", true
	case "time.Time":
		return "Time", true
	case "[]string":
		return "Strings", true
	case "[]int":
		return "Ints", true
	case "[]int64":
		return "Int64s", true
	case "[]bool":
		return "Bools", true
	case "[]float64":
		return "Float64s", true
	case "[]time.Time":
		return "Times", true
	default:
		return "", false
	}
}

func mvcScalarParameterTypeSuffix(typ string) (string, bool) {
	switch strings.TrimSpace(typ) {
	case "string":
		return "String", true
	case "int":
		return "Int", true
	case "int64":
		return "Int64", true
	case "bool":
		return "Bool", true
	case "float64":
		return "Float64", true
	case "time.Time":
		return "Time", true
	default:
		return "", false
	}
}

func hasMVCControllerAnnotation(annotations []Annotation) bool {
	return mvcControllerKind(annotations) != ""
}

func mvcControllerKind(annotations []Annotation) string {
	for _, name := range []string{"controller", "rest-controller", "mvc-controller"} {
		if hasAnnotation(annotations, name) {
			return name
		}
	}
	return ""
}

func hasMVCRouteMappingAnnotation(annotations []Annotation) bool {
	for _, annotation := range annotations {
		if isMVCRouteMappingAnnotation(annotation.Name) {
			return true
		}
	}
	return false
}

func isMVCRouteAnnotation(name string) bool {
	return isMVCRouteMappingAnnotation(name) || isMVCBodyAnnotation(name) || isMVCRequestEntityAnnotation(name) || isMVCMultipartBodyAnnotation(name) || isMVCParameterAnnotation(name) || isMVCValidatedAnnotation(name) || isMVCResponseBodyAnnotation(name) || isMVCResponseStatusAnnotation(name) || isMVCCrossOriginAnnotation(name)
}

func isMVCRouteMappingAnnotation(name string) bool {
	switch name {
	case "request-mapping", "get", "head", "post", "put", "patch", "delete", "options", "trace":
		return true
	default:
		return false
	}
}

func isMVCBodyAnnotation(name string) bool {
	switch name {
	case "request-body", "body":
		return true
	default:
		return false
	}
}

func isMVCMultipartBodyAnnotation(name string) bool {
	return name == "multipart-body"
}

func isMVCValidatedAnnotation(name string) bool {
	return name == "validated"
}

func isMVCParameterAnnotation(name string) bool {
	switch name {
	case "path-variable", "request-param", "request-header", "cookie-value", "model-attribute",
		"request-attribute", "session-attribute", "matrix-variable", "request-part":
		return true
	default:
		return false
	}
}

func isMVCResponseStatusAnnotation(name string) bool {
	return name == "response-status"
}

func hasMVCResponseBodyAnnotation(annotations []Annotation) bool {
	return hasAnnotation(annotations, "response-body")
}

func hasMVCValidatedAnnotation(annotations []Annotation) bool {
	return hasAnnotation(annotations, "validated")
}

func isMVCResponseBodyAnnotation(name string) bool {
	return name == "response-body"
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

func mvcMultipartBodySelectorSet(annotations []Annotation) map[string]struct{} {
	selectors := mvcMultipartBodySelectors(annotations)
	out := make(map[string]struct{}, len(selectors))
	for _, selector := range selectors {
		out[selector] = struct{}{}
	}
	return out
}

func mvcMultipartBodySelectors(annotations []Annotation) []string {
	selectors := make([]string, 0, 1)
	for _, annotation := range annotations {
		if !isMVCMultipartBodyAnnotation(annotation.Name) {
			continue
		}
		if selector := mvcMultipartBodySelector(annotation); selector != "" {
			selectors = append(selectors, selector)
		}
	}
	return selectors
}

func mvcMultipartBodySelector(annotation Annotation) string {
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

type mvcParameterBindingItem struct {
	Kind    mvcHandlerParamKind
	Binding mvcParamBinding
}

func mvcParameterBindingSet(annotations []Annotation) (map[string]mvcParameterBindingItem, error) {
	out := make(map[string]mvcParameterBindingItem)
	for _, annotation := range annotations {
		kind, ok := mvcParameterKind(annotation.Name)
		if !ok {
			continue
		}
		selector := mvcBindingSelector(annotation)
		if selector == "" {
			continue
		}
		if _, exists := out[selector]; exists {
			return nil, fmt.Errorf("mvc parameter %q has multiple binding annotations", selector)
		}
		out[selector] = mvcParameterBindingItem{
			Kind: kind,
			Binding: mvcParamBinding{
				SourceName:     mvcParameterSourceName(annotation, selector),
				SourceExplicit: mvcParameterHasSourceName(annotation),
				Required:       mvcParameterRequired(annotation),
				HasDefault:     mvcParameterHasDefault(annotation),
				DefaultValue:   mvcParameterDefaultValue(annotation),
			},
		}
	}
	return out, nil
}

func mvcParameterKind(name string) (mvcHandlerParamKind, bool) {
	switch name {
	case "path-variable":
		return mvcParamPathVariable, true
	case "request-param":
		return mvcParamRequestParam, true
	case "request-header":
		return mvcParamRequestHeader, true
	case "cookie-value":
		return mvcParamCookieValue, true
	case "model-attribute":
		return mvcParamModelAttribute, true
	case "request-attribute":
		return mvcParamRequestAttribute, true
	case "session-attribute":
		return mvcParamSessionAttribute, true
	case "matrix-variable":
		return mvcParamMatrixVariable, true
	case "request-part":
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

func mvcTypeBasePaths(annotations []Annotation) []string {
	for _, annotation := range annotations {
		if annotation.Name != "request-mapping" {
			continue
		}
		paths, err := requireMVCPathTexts(annotation)
		if err == nil {
			return normalizeMVCPaths(paths)
		}
	}
	return []string{""}
}

func requireMVCPath(annotation Annotation) error {
	_, err := requireMVCPathTexts(annotation)
	return err
}

func requireMVCPathTexts(annotation Annotation) ([]string, error) {
	values := annotationValueTexts(annotation)
	if len(values) == 0 {
		if value := argString(annotation, "path", ""); value != "" {
			values = []string{value}
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("annotation %q requires path value", annotation.Name)
	}
	paths := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("annotation %q requires path value", annotation.Name)
		}
		paths = append(paths, value)
	}
	return paths, nil
}

func mvcHTTPMethods(annotation Annotation) ([]string, error) {
	switch annotation.Name {
	case "get":
		return []string{http.MethodGet}, nil
	case "head":
		return []string{http.MethodHead}, nil
	case "post":
		return []string{http.MethodPost}, nil
	case "put":
		return []string{http.MethodPut}, nil
	case "patch":
		return []string{http.MethodPatch}, nil
	case "delete":
		return []string{http.MethodDelete}, nil
	case "options":
		return []string{http.MethodOptions}, nil
	case "trace":
		return []string{http.MethodTrace}, nil
	case "request-mapping":
		methods := strings.TrimSpace(argString(annotation, "method", ""))
		if methods == "" {
			return append([]string(nil), defaultMVCRequestMappingMethods[:]...), nil
		}
		return parseMVCRequestMethods(annotation, methods)
	default:
		return nil, fmt.Errorf("annotation %q requires supported http method", annotation.Name)
	}
}

func parseMVCRequestMethods(annotation Annotation, value string) ([]string, error) {
	parts := strings.Split(value, ",")
	methods := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		method := strings.ToUpper(strings.TrimSpace(part))
		if method == "" {
			return nil, fmt.Errorf("annotation %q requires supported http method", annotation.Name)
		}
		if !isSupportedMVCRequestMethod(method) {
			return nil, fmt.Errorf("annotation %q requires supported http method", annotation.Name)
		}
		if _, exists := seen[method]; exists {
			continue
		}
		seen[method] = struct{}{}
		methods = append(methods, method)
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("annotation %q requires supported http method", annotation.Name)
	}
	return methods, nil
}

func isSupportedMVCRequestMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func mvcStatus(annotation Annotation, fallback int) (int, error) {
	value := firstNonEmpty(argString(annotation, "status", ""), argString(annotation, "statusCode", ""))
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return parseMVCStatus(annotation, "status", value)
}

func mvcResponseStatus(annotation Annotation) (int, error) {
	values := annotationValueTexts(annotation)
	if len(values) > 1 {
		return 0, fmt.Errorf("annotation %q accepts exactly one status value", annotation.Name)
	}
	value := ""
	if len(values) == 1 {
		value = values[0]
	}
	namedValues := mvcNamedStatusValues(annotation, "status", "statusCode", "code")
	if len(namedValues) > 1 {
		return 0, fmt.Errorf("annotation %q accepts exactly one status argument", annotation.Name)
	}
	if len(namedValues) == 1 {
		if strings.TrimSpace(value) != "" {
			return 0, fmt.Errorf("annotation %q accepts either value or named status argument", annotation.Name)
		}
		value = namedValues[0]
	}
	if strings.TrimSpace(value) == "" {
		return 0, fmt.Errorf("annotation %q requires status value", annotation.Name)
	}
	return parseMVCStatus(annotation, "status", value)
}

func mvcNamedStatusValues(annotation Annotation, keys ...string) []string {
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func mvcMappingHasExplicitStatus(annotation Annotation) bool {
	for _, key := range []string{"status", "statusCode"} {
		if strings.TrimSpace(argString(annotation, key, "")) != "" {
			return true
		}
	}
	return false
}

func parseMVCStatus(annotation Annotation, label string, value string) (int, error) {
	status, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("annotation %q %s requires integer value: %w", annotation.Name, label, err)
	}
	if status < 100 || status > 999 {
		return 0, fmt.Errorf("annotation %q %s %d is out of range", annotation.Name, label, status)
	}
	return status, nil
}

func defaultMVCStatus(methods []string) int {
	if len(methods) == 1 && methods[0] == http.MethodPost {
		return http.StatusCreated
	}
	return http.StatusOK
}

func routeConstructor(method string) string {
	switch method {
	case http.MethodGet:
		return "GET"
	case http.MethodHead:
		return "HEAD"
	case http.MethodPost:
		return "POST"
	case http.MethodPut:
		return "PUT"
	case http.MethodPatch:
		return "PATCH"
	case http.MethodDelete:
		return "DELETE"
	case http.MethodOptions:
		return "OPTIONS"
	case http.MethodTrace:
		return "TRACE"
	default:
		return "Handle"
	}
}

func normalizeMVCPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(path, "/")
}

func normalizeMVCPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = normalizeMVCPath(path)
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func expandMVCRoutePaths(controller *mvcController, route mvcRoute) []mvcRoute {
	basePaths := controller.BasePaths
	if len(basePaths) == 0 {
		basePaths = []string{""}
	}
	paths := route.Paths
	if len(paths) == 0 {
		paths = []string{route.Path}
	}
	methods := route.HTTPMethods
	if len(methods) == 0 {
		methods = []string{route.HTTPMethod}
	}
	out := make([]mvcRoute, 0, len(methods)*len(basePaths)*len(paths))
	seen := make(map[string]struct{}, len(methods)*len(basePaths)*len(paths))
	for _, method := range methods {
		for _, basePath := range basePaths {
			for _, path := range paths {
				next := route
				next.HTTPMethod = method
				next.HTTPMethods = nil
				next.Path = joinMVCPaths(basePath, path)
				next.Paths = nil
				next.ControllerKind = controller.Kind
				key := next.HTTPMethod + "\x00" + next.Path
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, next)
			}
		}
	}
	return out
}

func joinMVCPaths(base string, path string) string {
	base = normalizeMVCPath(base)
	path = normalizeMVCPath(path)
	if base == "/" {
		return path
	}
	if path == "/" {
		return base
	}
	return base + path
}

func isArkWebContextExpr(file *ast.File, expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	return isImportedSelectorExpr(file, star.X, arkartaWebImportPath, "Context")
}

func isGoarkMVCModelPointerExpr(file *ast.File, expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	return isImportedSelectorExpr(file, star.X, goarkMVCImportPath, "Model")
}

func isSelectorTypeExpr(expr ast.Expr, selectorName string) bool {
	star, ok := expr.(*ast.StarExpr)
	if ok {
		expr = star.X
	}
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == selectorName
}

func isErrorExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "error"
}

func isArkWebResultExpr(file *ast.File, expr ast.Expr) bool {
	return isImportedSelectorExpr(file, expr, arkartaWebImportPath, "Result")
}

func isGoarkWebResponseEntityExpr(file *ast.File, expr ast.Expr) bool {
	switch typ := expr.(type) {
	case *ast.IndexExpr:
		return isImportedSelectorExpr(file, typ.X, goarkWebImportPath, "ResponseEntity")
	case *ast.IndexListExpr:
		return isImportedSelectorExpr(file, typ.X, goarkWebImportPath, "ResponseEntity")
	default:
		return false
	}
}

func isGoarkWebDownloadResultExpr(file *ast.File, expr ast.Expr) bool {
	return isImportedSelectorExpr(file, expr, goarkWebImportPath, "DownloadResult")
}

func isImportedSelectorExpr(file *ast.File, expr ast.Expr, importPath string, selectorName string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != selectorName {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	aliases := importAliases(file, importPath)
	_, ok = aliases[ident.Name]
	return ok
}

func importAliases(file *ast.File, importPath string) map[string]struct{} {
	aliases := make(map[string]struct{}, 1)
	if file == nil {
		return aliases
	}
	for _, spec := range file.Imports {
		if spec.Path == nil {
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != importPath {
			continue
		}
		if spec.Name == nil {
			aliases[defaultImportName(importPath)] = struct{}{}
			continue
		}
		switch spec.Name.Name {
		case "", "_", ".":
			continue
		default:
			aliases[spec.Name.Name] = struct{}{}
		}
	}
	return aliases
}

func defaultImportName(importPath string) string {
	importPath = strings.Trim(importPath, "/")
	if importPath == "" {
		return ""
	}
	index := strings.LastIndex(importPath, "/")
	if index < 0 {
		return importPath
	}
	return importPath[index+1:]
}

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

func mvcModelUsesConfigurer(model *mvcAnnotationModel) bool {
	if len(model.Controllers) > 0 {
		return true
	}
	for _, advice := range model.Advices {
		if len(advice.ExceptionHandlers) > 0 {
			return true
		}
	}
	return false
}

func mvcModelUsesArkWeb(model *mvcAnnotationModel) bool {
	for _, controller := range model.Controllers {
		if len(controller.Routes) > 0 {
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
