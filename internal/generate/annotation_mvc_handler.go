package generate

import (
	"fmt"
	"go/ast"
	"go/token"
)

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

func mvcModelAttributeMethodParams(file *ast.File, fn *ast.FuncDecl) ([]mvcHandlerParam, error) {
	if fn == nil || fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return nil, nil
	}
	params := make([]mvcHandlerParam, 0, len(fn.Type.Params.List))
	contextSeen := false
	for index, field := range fn.Type.Params.List {
		names := field.Names
		if len(names) == 0 {
			names = []*ast.Ident{ast.NewIdent(fmt.Sprintf("arg%d", index))}
		}
		if len(names) != 1 {
			return nil, fmt.Errorf("mvc model attribute method %s parameter group must declare exactly one name", fn.Name.Name)
		}
		name := names[0].Name
		if isArkWebContextExpr(file, field.Type) {
			if contextSeen {
				return nil, fmt.Errorf("mvc model attribute method %s must not declare multiple *arkarta/web.Context parameters", fn.Name.Name)
			}
			contextSeen = true
			params = append(params, mvcHandlerParam{Name: name, Kind: mvcParamContext})
			continue
		}
		if isSelectorTypeExpr(field.Type, "Context") {
			return nil, fmt.Errorf("mvc model attribute method %s parameter must be *arkarta/web.Context", fn.Name.Name)
		}
		return nil, fmt.Errorf("mvc model attribute method %s parameter %s must be *arkarta/web.Context", fn.Name.Name, name)
	}
	return params, nil
}

func mvcModelAttributeMethodSupportsReturn(kind mvcReturnKind) bool {
	switch kind {
	case mvcReturnValue, mvcReturnValueError:
		return true
	default:
		return false
	}
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
