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
	mvcAnnotationModelKey = "goark.mvc.annotations"
	arkartaWebImportPath  = "goark.dev/arkarta/web"
)

type mvcAnnotationModel struct {
	Controllers []*mvcController
	byType      map[string]*mvcController
	pending     []mvcRoute
}

type mvcController struct {
	Component annotationComponent
	BasePath  string
	Routes    []mvcRoute
}

type mvcRoute struct {
	ControllerType string
	MethodName     string
	HTTPMethod     string
	Path           string
	Status         int
	Handler        mvcHandler
}

type mvcHandler struct {
	Params     []mvcHandlerParam
	ReturnKind mvcReturnKind
}

type mvcHandlerParam struct {
	Name    string
	Type    string
	Kind    mvcHandlerParamKind
	Binding mvcParamBinding
}

type mvcParamBinding struct {
	SourceName   string
	Required     bool
	HasDefault   bool
	DefaultValue string
}

type mvcHandlerParamKind uint8

const (
	mvcParamContext mvcHandlerParamKind = iota + 1
	mvcParamBody
	mvcParamPathVariable
	mvcParamRequestParam
	mvcParamRequestHeader
	mvcParamCookieValue
)

type mvcReturnKind uint8

const (
	mvcReturnNone mvcReturnKind = iota
	mvcReturnError
	mvcReturnResult
	mvcReturnResultError
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
		{Name: "request-mapping", Targets: []AnnotationTarget{AnnotationTargetType, AnnotationTargetMethod}, Validate: validateMVCRequestMappingAnnotation},
		{Name: "get", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "post", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "put", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "patch", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "delete", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCHTTPMappingAnnotation},
		{Name: "request-body", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCRequestBodyAnnotation},
		{Name: "body", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCRequestBodyAnnotation},
		{Name: "path-variable", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCParameterBindingAnnotation},
		{Name: "request-param", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCParameterBindingAnnotation},
		{Name: "request-header", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCParameterBindingAnnotation},
		{Name: "cookie-value", Targets: []AnnotationTarget{AnnotationTargetMethod}, Validate: validateMVCParameterBindingAnnotation},
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
		return bindMVCController(ctx, item)
	case AnnotationTargetMethod:
		return bindMVCRoute(ctx, item)
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
		route.Path = joinMVCPaths(controller.BasePath, route.Path)
		controller.Routes = append(controller.Routes, route)
	}
	coreModel := ensureCoreAnnotationModel(ctx)
	resolver := newAnnotationDependencyResolver(coreModel)
	for _, controller := range model.Controllers {
		resolver.addCandidate(annotationDependencyCandidate{
			Name: controller.Component.Name,
			Type: "*" + controller.Component.TypeName,
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
	sort.SliceStable(model.Controllers, func(i, j int) bool {
		return model.Controllers[i].Component.Name < model.Controllers[j].Component.Name
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
	model := &mvcAnnotationModel{byType: make(map[string]*mvcController)}
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
	if len(model.Controllers) == 0 {
		return nil
	}
	ctx.AddImport("arkweb", arkartaWebImportPath)
	ctx.AddImport("goweb", "goark.dev/goark/web")
	ctx.AddImport("", "goark.dev/goark/web/mvc")
	if mvcModelUsesOptionalInjection(model) {
		ctx.AddImport("arkerrors", "goark.dev/goark/errors")
	}
	writeMVCConfiguration(ctx.buffer(), model)
	return nil
}

func buildMVCController(fset *token.FileSet, typeSpec *ast.TypeSpec, annotations []Annotation) (*mvcController, error) {
	typeName := typeSpec.Name.Name
	component := annotationComponent{
		TypeName: typeName,
		Name:     annotationName(annotations, mvcControllerKind(annotations), lowerCamel(typeName)),
	}
	structType, _ := typeSpec.Type.(*ast.StructType)
	if structType != nil {
		for _, field := range structType.Fields.List {
			fieldAnnotations, err := parseAnnotations(field.Doc)
			if err != nil {
				return nil, err
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
	}
	return &mvcController{
		Component: component,
		BasePath:  mvcTypeBasePath(annotations),
	}, nil
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
		HTTPMethod: mapping.method,
		Path:       mapping.path,
		Status:     mapping.status,
		Handler:    handler,
	}, nil
}

type mvcRouteMappingSpec struct {
	method string
	path   string
	status int
}

func mvcRouteFromAnnotations(annotations []Annotation) (mvcRouteMappingSpec, error) {
	var out mvcRouteMappingSpec
	for _, annotation := range annotations {
		if !isMVCRouteMappingAnnotation(annotation.Name) {
			continue
		}
		if out.method != "" {
			return mvcRouteMappingSpec{}, fmt.Errorf("mvc route method has multiple mapping annotations")
		}
		mapping, err := mvcRouteMapping(annotation)
		if err != nil {
			return mvcRouteMappingSpec{}, err
		}
		out = mapping
	}
	if out.method == "" {
		return mvcRouteMappingSpec{}, fmt.Errorf("mvc route method requires mapping annotation")
	}
	return out, nil
}

func mvcRouteMapping(annotation Annotation) (mvcRouteMappingSpec, error) {
	path, err := requireMVCPathText(annotation)
	if err != nil {
		return mvcRouteMappingSpec{}, err
	}
	method := mvcHTTPMethod(annotation)
	if method == "" {
		return mvcRouteMappingSpec{}, fmt.Errorf("annotation %q requires supported http method", annotation.Name)
	}
	status, err := mvcStatus(annotation, defaultMVCStatus(method))
	if err != nil {
		return mvcRouteMappingSpec{}, err
	}
	return mvcRouteMappingSpec{
		method: method,
		path:   normalizeMVCPath(path),
		status: status,
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
	if hasMVCBodyParam(params) && returnKind != mvcReturnValue && returnKind != mvcReturnValueError {
		return mvcHandler{}, fmt.Errorf("mvc handler method %s with request body must return T or T,error", fn.Name.Name)
	}
	return mvcHandler{Params: params, ReturnKind: returnKind}, nil
}

func mvcMethodParams(fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, annotations []Annotation) ([]mvcHandlerParam, error) {
	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		if len(mvcRequestBodySelectors(annotations)) > 0 {
			return nil, fmt.Errorf("mvc handler method %s request body selector does not match any method parameter", fn.Name.Name)
		}
		return nil, nil
	}

	bodySelectors := mvcRequestBodySelectorSet(annotations)
	paramBindings, err := mvcParameterBindingSet(annotations)
	if err != nil {
		return nil, err
	}
	params := make([]mvcHandlerParam, 0, len(fn.Type.Params.List))
	contextSeen := false
	bodySeen := false
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
		if isSelectorTypeExpr(field.Type, "Context") {
			return nil, fmt.Errorf("mvc handler method %s parameter must be *arkarta/web.Context", fn.Name.Name)
		}
		if _, isBody := bodySelectors[name]; isBody {
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
		if binding, isBound := paramBindings[name]; isBound {
			typ := exprString(fset, field.Type)
			if _, ok := mvcParameterBindingCall(mvcHandlerParam{Type: typ, Kind: binding.Kind, Binding: binding.Binding}); !ok {
				return nil, fmt.Errorf("mvc handler method %s parameter %s has unsupported mvc parameter type %s", fn.Name.Name, name, typ)
			}
			params = append(params, mvcHandlerParam{Name: name, Type: typ, Kind: binding.Kind, Binding: binding.Binding})
			continue
		}
		return nil, fmt.Errorf("mvc handler method %s parameter %s must be *arkarta/web.Context or annotated with mvc binding annotation", fn.Name.Name, name)
	}
	for selector := range bodySelectors {
		if !mvcHasParam(params, selector) {
			return nil, fmt.Errorf("mvc handler method %s request body selector %q does not match any method parameter", fn.Name.Name, selector)
		}
	}
	for selector := range paramBindings {
		if !mvcHasParam(params, selector) {
			return nil, fmt.Errorf("mvc handler method %s parameter binding selector %q does not match any method parameter", fn.Name.Name, selector)
		}
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
		default:
			return mvcReturnValue, nil
		}
	}
	if len(results.List) == 2 && isErrorExpr(results.List[1].Type) {
		if isArkWebResultExpr(file, results.List[0].Type) {
			return mvcReturnResultError, nil
		}
		return mvcReturnValueError, nil
	}
	return 0, fmt.Errorf("mvc handler method %s must return void, error, T, T,error, web.Result, or web.Result,error", fn.Name.Name)
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
	builder.WriteString("out = mvc.NewConfigurer(mvc.NewController(")
	builder.WriteString(strconv.Quote(controller.Component.Name))
	for _, route := range controller.Routes {
		builder.WriteString(",\n")
		writeMVCRoute(builder, route)
	}
	builder.WriteString("))\nreturn out, nil\n}, container.WithFactoryDependencies(")
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
	builder.WriteByte(')')
}

func writeMVCHandler(builder *bytes.Buffer, route mvcRoute) {
	if hasMVCBodyParam(route.Handler.Params) {
		writeMVCBindJSONHandler(builder, route)
		return
	}
	call := mvcHandlerCall(route.MethodName, route.Handler.Params)
	switch route.Handler.ReturnKind {
	case mvcReturnResultError:
		builder.WriteString("mvc.Handler(func(ctx *arkweb.Context) (arkweb.Result, error) {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err")
		builder.WriteString("return ")
		builder.WriteString(call)
		builder.WriteString("\n})")
	case mvcReturnResult:
		builder.WriteString("mvc.Handler(func(ctx *arkweb.Context) (arkweb.Result, error) {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err")
		builder.WriteString("return ")
		builder.WriteString(call)
		builder.WriteString(", nil\n})")
	case mvcReturnValueError:
		builder.WriteString("mvc.JSON[any](")
		builder.WriteString(strconv.Itoa(route.Status))
		builder.WriteString(", func(ctx *arkweb.Context) (any, error) {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err")
		builder.WriteString("return ")
		builder.WriteString(call)
		builder.WriteString("\n})")
	case mvcReturnValue:
		builder.WriteString("mvc.JSON[any](")
		builder.WriteString(strconv.Itoa(route.Status))
		builder.WriteString(", func(ctx *arkweb.Context) (any, error) {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err")
		builder.WriteString("return ")
		builder.WriteString(call)
		builder.WriteString(", nil\n})")
	case mvcReturnError:
		builder.WriteString("mvc.NoContent(func(ctx *arkweb.Context) error {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return err")
		builder.WriteString("return ")
		builder.WriteString(call)
		builder.WriteString("\n})")
	default:
		builder.WriteString("mvc.NoContent(func(ctx *arkweb.Context) error {\n")
		writeMVCParameterBindings(builder, route.Handler.Params, "return err")
		builder.WriteString(call)
		builder.WriteString("\nreturn nil\n})")
	}
}

func writeMVCBindJSONHandler(builder *bytes.Buffer, route mvcRoute) {
	bodyParam, _ := mvcBodyParam(route.Handler.Params)
	builder.WriteString("mvc.BindJSON[")
	builder.WriteString(bodyParam.Type)
	builder.WriteString(", any](")
	builder.WriteString(strconv.Itoa(route.Status))
	builder.WriteString(", func(ctx *arkweb.Context, ")
	builder.WriteString(bodyParam.Name)
	builder.WriteByte(' ')
	builder.WriteString(bodyParam.Type)
	builder.WriteString(") (any, error) {\n")
	writeMVCParameterBindings(builder, route.Handler.Params, "return nil, err")
	builder.WriteString("return ")
	builder.WriteString(mvcHandlerCall(route.MethodName, route.Handler.Params))
	if route.Handler.ReturnKind == mvcReturnValue {
		builder.WriteString(", nil")
	}
	builder.WriteString("\n})")
}

func mvcHandlerCall(methodName string, params []mvcHandlerParam) string {
	args := make([]string, 0, len(params))
	for _, param := range params {
		switch param.Kind {
		case mvcParamContext:
			args = append(args, "ctx")
		case mvcParamBody:
			args = append(args, param.Name)
		case mvcParamPathVariable, mvcParamRequestParam, mvcParamRequestHeader, mvcParamCookieValue:
			args = append(args, param.Name)
		}
	}
	return "controller." + methodName + "(" + strings.Join(args, ", ") + ")"
}

func writeMVCParameterBindings(builder *bytes.Buffer, params []mvcHandlerParam, errorReturn string) {
	for _, param := range params {
		call, ok := mvcParameterBindingCall(param)
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

func mvcParameterBindingCall(param mvcHandlerParam) (string, bool) {
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
	suffix, ok := mvcParameterTypeSuffix(typ)
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
	default:
		return "", false
	}
}

func mvcParameterTypeSuffix(typ string) (string, bool) {
	switch strings.TrimSpace(typ) {
	case "string":
		return "String", true
	case "int":
		return "Int", true
	case "int64":
		return "Int64", true
	case "bool":
		return "Bool", true
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
	return isMVCRouteMappingAnnotation(name) || isMVCBodyAnnotation(name) || isMVCParameterAnnotation(name)
}

func isMVCRouteMappingAnnotation(name string) bool {
	switch name {
	case "request-mapping", "get", "post", "put", "patch", "delete":
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

func isMVCParameterAnnotation(name string) bool {
	switch name {
	case "path-variable", "request-param", "request-header", "cookie-value":
		return true
	default:
		return false
	}
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
				SourceName:   mvcParameterSourceName(annotation, selector),
				Required:     mvcParameterRequired(annotation),
				HasDefault:   mvcParameterHasDefault(annotation),
				DefaultValue: mvcParameterDefaultValue(annotation),
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
	for _, key := range []string{"name", "value"} {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			return value
		}
	}
	return fallback
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

func mvcBodyParam(params []mvcHandlerParam) (mvcHandlerParam, bool) {
	for _, param := range params {
		if param.Kind == mvcParamBody {
			return param, true
		}
	}
	return mvcHandlerParam{}, false
}

func mvcTypeBasePath(annotations []Annotation) string {
	for _, annotation := range annotations {
		if annotation.Name != "request-mapping" {
			continue
		}
		path, err := requireMVCPathText(annotation)
		if err == nil {
			return normalizeMVCPath(path)
		}
	}
	return ""
}

func requireMVCPath(annotation Annotation) error {
	_, err := requireMVCPathText(annotation)
	return err
}

func requireMVCPathText(annotation Annotation) (string, error) {
	values := annotationValueTexts(annotation)
	if len(values) == 0 {
		if value := argString(annotation, "path", ""); value != "" {
			values = []string{value}
		}
	}
	if len(values) == 0 {
		return "", fmt.Errorf("annotation %q requires path value", annotation.Name)
	}
	if len(values) > 1 {
		return "", fmt.Errorf("annotation %q accepts exactly one path value", annotation.Name)
	}
	value := strings.TrimSpace(values[0])
	if value == "" {
		return "", fmt.Errorf("annotation %q requires path value", annotation.Name)
	}
	return value, nil
}

func mvcHTTPMethod(annotation Annotation) string {
	switch annotation.Name {
	case "get":
		return http.MethodGet
	case "post":
		return http.MethodPost
	case "put":
		return http.MethodPut
	case "patch":
		return http.MethodPatch
	case "delete":
		return http.MethodDelete
	case "request-mapping":
		method := strings.ToUpper(strings.TrimSpace(argString(annotation, "method", "")))
		switch method {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			return method
		default:
			return ""
		}
	default:
		return ""
	}
}

func mvcStatus(annotation Annotation, fallback int) (int, error) {
	value := firstNonEmpty(argString(annotation, "status", ""), argString(annotation, "statusCode", ""))
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	status, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("annotation %q status requires integer value: %w", annotation.Name, err)
	}
	if status < 100 || status > 999 {
		return 0, fmt.Errorf("annotation %q status %d is out of range", annotation.Name, status)
	}
	return status, nil
}

func defaultMVCStatus(method string) int {
	if method == http.MethodPost {
		return http.StatusCreated
	}
	return http.StatusOK
}

func routeConstructor(method string) string {
	switch method {
	case http.MethodGet:
		return "GET"
	case http.MethodPost:
		return "POST"
	case http.MethodPut:
		return "PUT"
	case http.MethodPatch:
		return "PATCH"
	case http.MethodDelete:
		return "DELETE"
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
	return false
}
