package generate

import (
	"fmt"
	"go/ast"
	"go/token"
	"net/http"
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
	pendingModelAttributes   []mvcModelAttributeMethod
	pendingExceptionHandlers []mvcExceptionHandler
}

type mvcController struct {
	Component annotationComponent
	BasePaths []string
	Methods   []string
	Routes    []mvcRoute
	Kind      string

	Conditions  mvcRouteConditions
	CrossOrigin *mvcCrossOrigin

	ModelAttributes []mvcModelAttributeMethod
}

type mvcRoute struct {
	ControllerType   string
	MethodName       string
	HTTPMethod       string
	HTTPMethods      []string
	HTTPMethodsSet   bool
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

type mvcModelAttributeMethod struct {
	ControllerType string
	MethodName     string
	Name           string
	Params         []mvcHandlerParam
	ReturnKind     mvcReturnKind
	ReturnType     string
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
type mvcControllerAdvice struct {
	Component         annotationComponent
	ExceptionHandlers []mvcExceptionHandler
	Kind              string
}
type mvcExceptionHandler struct {
	AdviceType   string
	MethodName   string
	ErrorType    string
	Params       []mvcExceptionHandlerParam
	ReturnKind   mvcReturnKind
	EntityBody   string
	Status       int
	ResponseBody bool
}

func validateMVCControllerAdviceAnnotation(ctx AnnotationValidationContext) error {
	typeSpec := ctx.Item.TypeSpec()
	if typeSpec == nil {
		return annotationError("requires type target", ctx.Annotation.Name)
	}
	if _, ok := typeSpec.Type.(*ast.StructType); !ok {
		return annotationError("requires struct type target", ctx.Annotation.Name)
	}
	if hasMVCControllerAnnotation(ctx.Item.Annotations()) {
		return annotationError("target must not also declare mvc controller", ctx.Annotation.Name)
	}
	return validateCoreNameAnnotation(ctx.Annotation)
}

func validateMVCExceptionHandlerAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return annotationError("must not be combined with mvc route mapping", ctx.Annotation.Name)
	}
	if len(ctx.Annotation.Args) > 0 || len(ctx.Annotation.Values) > 0 {
		return annotationError("does not accept arguments", ctx.Annotation.Name)
	}
	selector := normalizeSelector(ctx.Annotation.Selector)
	if selector != "" && !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return annotationSelectorError(ctx.Annotation.Name, selector)
	}
	return nil
}

func bindMVCControllerAdvice(ctx *AnnotationBindingContext, item AnnotationItem) error {
	if !hasMVCControllerAdviceAnnotation(item.Annotations()) {
		return nil
	}
	typeSpec := item.TypeSpec()
	if typeSpec == nil {
		return nil
	}
	advice, err := buildMVCControllerAdvice(item.FileSet(), typeSpec, item.Annotations())
	if err != nil {
		return err
	}
	model := ensureMVCAnnotationModel(ctx)
	if _, exists := model.adviceByType[advice.Component.TypeName]; exists {
		return fmt.Errorf("duplicate mvc controller advice type %q", advice.Component.TypeName)
	}
	model.Advices = append(model.Advices, advice)
	model.adviceByType[advice.Component.TypeName] = advice
	return nil
}

func bindMVCExceptionHandler(ctx *AnnotationBindingContext, item AnnotationItem) error {
	if !hasMVCExceptionHandlerAnnotation(item.Annotations()) {
		return nil
	}
	handler, err := buildMVCExceptionHandler(
		item.FileSet(), item.File(), item.FuncDecl(), item.Annotations(),
	)
	if err != nil {
		return err
	}
	handler.AdviceType = item.ReceiverTypeName()
	handler.MethodName = item.FuncName()
	model := ensureMVCAnnotationModel(ctx)
	model.pendingExceptionHandlers = append(model.pendingExceptionHandlers, handler)
	return nil
}

func buildMVCExceptionHandler(
	fset *token.FileSet, file *ast.File,
	fn *ast.FuncDecl, annotations []Annotation,
) (mvcExceptionHandler, error) {
	if fn == nil {
		return mvcExceptionHandler{}, fmt.Errorf("mvc exception handler method is nil")
	}
	params, errorType, err := mvcExceptionHandlerParams(fset, file, fn, annotations)
	if err != nil {
		return mvcExceptionHandler{}, err
	}
	returnKind, entityBody, err := mvcExceptionHandlerReturn(fset, file, fn)
	if err != nil {
		return mvcExceptionHandler{}, err
	}
	status, responseBody, err := mvcExceptionHandlerResponseSpec(annotations, returnKind)
	if err != nil {
		return mvcExceptionHandler{}, err
	}
	return mvcExceptionHandler{
		ErrorType:    errorType,
		Params:       params,
		ReturnKind:   returnKind,
		EntityBody:   entityBody,
		Status:       status,
		ResponseBody: responseBody,
	}, nil
}

func mvcExceptionHandlerParams(
	fset *token.FileSet, file *ast.File,
	fn *ast.FuncDecl, annotations []Annotation,
) ([]mvcExceptionHandlerParam, string, error) {
	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return nil, "", mvcExceptionHandlerError(fn.Name.Name, "must declare error parameter")
	}
	selector := mvcExceptionHandlerSelector(annotations)
	params := make([]mvcExceptionHandlerParam, 0, len(fn.Type.Params.List))
	contextSeen := false
	errorSeen := false
	selectorMatchedError := selector == ""
	errorType := ""
	for index, field := range fn.Type.Params.List {
		names := field.Names
		if len(names) == 0 {
			names = []*ast.Ident{ast.NewIdent(fmt.Sprintf("arg%d", index))}
		}
		if len(names) != 1 {
			return nil, "", mvcExceptionHandlerError(fn.Name.Name, mvcErrParameterGroup)
		}
		name := names[0].Name
		if isArkWebContextExpr(file, field.Type) {
			if contextSeen {
				return nil, "", mvcExceptionHandlerError(fn.Name.Name, mvcErrMultipleContexts)
			}
			contextSeen = true
			params = append(params, mvcExceptionHandlerParam{Kind: mvcExceptionParamContext})
			continue
		}
		if isSelectorTypeExpr(field.Type, "Context") {
			return nil, "", mvcExceptionHandlerError(fn.Name.Name, mvcErrParameterType)
		}
		if errorSeen {
			return nil, "", mvcExceptionHandlerError(fn.Name.Name, mvcErrMultipleErrors)
		}
		errorSeen = true
		errorType = exprString(fset, field.Type)
		selectorMatchedError = selectorMatchedError || selector == name
		params = append(params, mvcExceptionHandlerParam{Kind: mvcExceptionParamError})
	}
	if !errorSeen {
		return nil, "", mvcExceptionHandlerError(fn.Name.Name, "must declare error parameter")
	}
	if !selectorMatchedError {
		return nil, "", mvcExceptionHandlerError(fn.Name.Name, mvcErrErrorSelector, selector)
	}
	return params, errorType, nil
}

func mvcExceptionHandlerReturn(
	fset *token.FileSet, file *ast.File, fn *ast.FuncDecl,
) (mvcReturnKind, string, error) {
	results := fn.Type.Results
	if results == nil || len(results.List) != 1 {
		return mvcReturnNone, "", mvcExceptionHandlerError(fn.Name.Name, mvcErrReturnType)
	}
	result := results.List[0].Type
	if isErrorExpr(result) {
		return mvcReturnNone, "", mvcExceptionHandlerError(fn.Name.Name, mvcErrReturnType)
	}
	if isArkWebResultExpr(file, result) || isGoarkWebDownloadResultExpr(file, result) {
		return mvcReturnResult, "", nil
	}
	if entityBody, ok := mvcResponseEntityBodyType(fset, file, result); ok {
		return mvcReturnEntity, entityBody, nil
	}
	return mvcReturnValue, "", nil
}

func mvcExceptionHandlerResponseSpec(
	annotations []Annotation, returnKind mvcReturnKind,
) (int, bool, error) {
	status := 0
	hasStatus := false
	responseBody := false
	for _, annotation := range annotations {
		switch {
		case isMVCResponseStatusAnnotation(annotation.Name):
			if hasStatus {
				return 0, false, fmt.Errorf(
					"mvc exception handler method has multiple response-status annotations",
				)
			}
			value, err := mvcResponseStatus(annotation)
			if err != nil {
				return 0, false, err
			}
			status = value
			hasStatus = true
		case isMVCResponseBodyAnnotation(annotation.Name):
			if responseBody {
				return 0, false, fmt.Errorf(
					"mvc exception handler method has multiple response-body annotations",
				)
			}
			responseBody = true
		}
	}
	if hasStatus && returnKind != mvcReturnValue {
		return 0, false, fmt.Errorf(
			"mvc exception handler response-status requires ordinary return value",
		)
	}
	if responseBody && returnKind != mvcReturnValue {
		return 0, false, fmt.Errorf("mvc exception handler response-body requires ordinary return value")
	}
	return status, responseBody, nil
}
