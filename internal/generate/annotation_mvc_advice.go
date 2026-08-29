package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

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

type mvcExceptionHandlerParam struct {
	Kind mvcExceptionHandlerParamKind
}

type mvcExceptionHandlerParamKind uint8

const (
	mvcExceptionParamContext mvcExceptionHandlerParamKind = iota + 1
	mvcExceptionParamError
)

func validateMVCControllerAdviceAnnotation(ctx AnnotationValidationContext) error {
	typeSpec := ctx.Item.TypeSpec()
	if typeSpec == nil {
		return fmt.Errorf("annotation %q requires type target", ctx.Annotation.Name)
	}
	if _, ok := typeSpec.Type.(*ast.StructType); !ok {
		return fmt.Errorf("annotation %q requires struct type target", ctx.Annotation.Name)
	}
	if hasMVCControllerAnnotation(ctx.Item.Annotations()) {
		return fmt.Errorf("annotation %q target must not also declare mvc controller", ctx.Annotation.Name)
	}
	return validateCoreNameAnnotation(ctx.Annotation)
}

func validateMVCExceptionHandlerAnnotation(ctx AnnotationValidationContext) error {
	if err := validateMVCHandlerMethod(ctx); err != nil {
		return err
	}
	if hasMVCRouteMappingAnnotation(ctx.Item.Annotations()) {
		return fmt.Errorf("annotation %q must not be combined with mvc route mapping", ctx.Annotation.Name)
	}
	if len(ctx.Annotation.Args) > 0 || len(ctx.Annotation.Values) > 0 {
		return fmt.Errorf("annotation %q does not accept arguments", ctx.Annotation.Name)
	}
	if selector := normalizeSelector(ctx.Annotation.Selector); selector != "" && !methodHasParameter(ctx.Item.FuncDecl(), selector) {
		return fmt.Errorf("annotation %q selector %q does not match any method parameter", ctx.Annotation.Name, selector)
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
	handler, err := buildMVCExceptionHandler(item.FileSet(), item.File(), item.FuncDecl(), item.Annotations())
	if err != nil {
		return err
	}
	handler.AdviceType = item.ReceiverTypeName()
	handler.MethodName = item.FuncName()
	model := ensureMVCAnnotationModel(ctx)
	model.pendingExceptionHandlers = append(model.pendingExceptionHandlers, handler)
	return nil
}

func buildMVCControllerAdvice(fset *token.FileSet, typeSpec *ast.TypeSpec, annotations []Annotation) (*mvcControllerAdvice, error) {
	component, err := buildMVCComponent(fset, typeSpec, annotations, mvcControllerAdviceKind(annotations))
	if err != nil {
		return nil, err
	}
	return &mvcControllerAdvice{
		Component: component,
		Kind:      mvcControllerAdviceKind(annotations),
	}, nil
}

func buildMVCExceptionHandler(fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, annotations []Annotation) (mvcExceptionHandler, error) {
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

func mvcExceptionHandlerParams(fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, annotations []Annotation) ([]mvcExceptionHandlerParam, string, error) {
	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return nil, "", fmt.Errorf("mvc exception handler method %s must declare error parameter", fn.Name.Name)
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
			return nil, "", fmt.Errorf("mvc exception handler method %s parameter group must declare exactly one name", fn.Name.Name)
		}
		name := names[0].Name
		if isArkWebContextExpr(file, field.Type) {
			if contextSeen {
				return nil, "", fmt.Errorf("mvc exception handler method %s must not declare multiple *arkarta/web.Context parameters", fn.Name.Name)
			}
			contextSeen = true
			params = append(params, mvcExceptionHandlerParam{Kind: mvcExceptionParamContext})
			continue
		}
		if isSelectorTypeExpr(field.Type, "Context") {
			return nil, "", fmt.Errorf("mvc exception handler method %s parameter must be *arkarta/web.Context or error type", fn.Name.Name)
		}
		if errorSeen {
			return nil, "", fmt.Errorf("mvc exception handler method %s must declare exactly one error parameter", fn.Name.Name)
		}
		errorSeen = true
		errorType = exprString(fset, field.Type)
		selectorMatchedError = selectorMatchedError || selector == name
		params = append(params, mvcExceptionHandlerParam{Kind: mvcExceptionParamError})
	}
	if !errorSeen {
		return nil, "", fmt.Errorf("mvc exception handler method %s must declare error parameter", fn.Name.Name)
	}
	if !selectorMatchedError {
		return nil, "", fmt.Errorf("mvc exception handler method %s selector %q must reference error parameter", fn.Name.Name, selector)
	}
	return params, errorType, nil
}

func mvcExceptionHandlerReturn(fset *token.FileSet, file *ast.File, fn *ast.FuncDecl) (mvcReturnKind, string, error) {
	results := fn.Type.Results
	if results == nil || len(results.List) != 1 {
		return mvcReturnNone, "", fmt.Errorf("mvc exception handler method %s must return arkarta/web.Result, web.ResponseEntity, or ordinary value", fn.Name.Name)
	}
	result := results.List[0].Type
	if isErrorExpr(result) {
		return mvcReturnNone, "", fmt.Errorf("mvc exception handler method %s must return arkarta/web.Result, web.ResponseEntity, or ordinary value", fn.Name.Name)
	}
	if isArkWebResultExpr(file, result) || isGoarkWebDownloadResultExpr(file, result) {
		return mvcReturnResult, "", nil
	}
	if entityBody, ok := mvcResponseEntityBodyType(fset, file, result); ok {
		return mvcReturnEntity, entityBody, nil
	}
	return mvcReturnValue, "", nil
}

func mvcExceptionHandlerResponseSpec(annotations []Annotation, returnKind mvcReturnKind) (int, bool, error) {
	status := 0
	hasStatus := false
	responseBody := false
	for _, annotation := range annotations {
		switch {
		case isMVCResponseStatusAnnotation(annotation.Name):
			if hasStatus {
				return 0, false, fmt.Errorf("mvc exception handler method has multiple response-status annotations")
			}
			value, err := mvcResponseStatus(annotation)
			if err != nil {
				return 0, false, err
			}
			status = value
			hasStatus = true
		case isMVCResponseBodyAnnotation(annotation.Name):
			if responseBody {
				return 0, false, fmt.Errorf("mvc exception handler method has multiple response-body annotations")
			}
			responseBody = true
		}
	}
	if hasStatus && returnKind != mvcReturnValue {
		return 0, false, fmt.Errorf("mvc exception handler response-status requires ordinary return value")
	}
	if responseBody && returnKind != mvcReturnValue {
		return 0, false, fmt.Errorf("mvc exception handler response-body requires ordinary return value")
	}
	return status, responseBody, nil
}

func mvcResponseEntityBodyType(fset *token.FileSet, file *ast.File, expr ast.Expr) (string, bool) {
	switch typ := expr.(type) {
	case *ast.IndexExpr:
		if !isImportedSelectorExpr(file, typ.X, goarkWebImportPath, "ResponseEntity") {
			return "", false
		}
		return exprString(fset, typ.Index), true
	case *ast.IndexListExpr:
		if !isImportedSelectorExpr(file, typ.X, goarkWebImportPath, "ResponseEntity") || len(typ.Indices) != 1 {
			return "", false
		}
		return exprString(fset, typ.Indices[0]), true
	default:
		return "", false
	}
}

func writeMVCAdviceConfigurerRegistration(builder *bytes.Buffer, advice *mvcControllerAdvice) {
	if len(advice.ExceptionHandlers) == 0 {
		return
	}
	configurerName := advice.Component.Name + ".mvcAdviceConfigurer"
	builder.WriteString("if err := container.Register[goweb.Configurer](registry, ")
	builder.WriteString(strconv.Quote(configurerName))
	builder.WriteString(", func(ctx context.Context, resolver container.Resolver) (out goweb.Configurer, err error) {\n")
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

func hasMVCControllerAdviceAnnotation(annotations []Annotation) bool {
	return mvcControllerAdviceKind(annotations) != ""
}

func mvcControllerAdviceKind(annotations []Annotation) string {
	for _, name := range []string{"controller-advice", "rest-controller-advice"} {
		if hasAnnotation(annotations, name) {
			return name
		}
	}
	return ""
}

func hasMVCExceptionHandlerAnnotation(annotations []Annotation) bool {
	for _, annotation := range annotations {
		if annotation.Name == "exception-handler" {
			return true
		}
	}
	return false
}

func mvcExceptionHandlerSelector(annotations []Annotation) string {
	for _, annotation := range annotations {
		if annotation.Name == "exception-handler" {
			return normalizeSelector(annotation.Selector)
		}
	}
	return ""
}

func mvcExceptionHandlerUsesContext(handler mvcExceptionHandler) bool {
	for _, param := range handler.Params {
		if param.Kind == mvcExceptionParamContext {
			return true
		}
	}
	return false
}
