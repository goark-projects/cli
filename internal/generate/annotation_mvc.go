package generate

import (
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
