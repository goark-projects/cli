package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
)

type mvcExceptionHandlerParam struct {
	Kind mvcExceptionHandlerParamKind
}

type mvcExceptionHandlerParamKind uint8

const (
	mvcExceptionParamContext mvcExceptionHandlerParamKind = iota + 1
	mvcExceptionParamError
)

func mvcMethodParams(
	fset *token.FileSet, file *ast.File,
	fn *ast.FuncDecl, annotations []Annotation,
) ([]mvcHandlerParam, error) {
	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		if len(mvcRequestBodySelectors(annotations)) > 0 {
			return nil, mvcMissingSelectorError(fn.Name.Name, "request body")
		}
		if len(mvcRequestEntitySelectors(annotations)) > 0 {
			return nil, mvcMissingSelectorError(fn.Name.Name, "request entity")
		}
		if len(mvcMultipartBodySelectors(annotations)) > 0 {
			return nil, mvcMissingSelectorError(fn.Name.Name, "multipart body")
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
			return nil, mvcHandlerError("parameter group must declare exactly one name", fn.Name.Name)
		}
		name := names[0].Name
		if isArkWebContextExpr(file, field.Type) {
			if _, isBody := bodySelectors[name]; isBody {
				return nil, mvcContextBindingError(fn.Name.Name, "request body", name)
			}
			if _, isRequestEntity := requestEntitySelectors[name]; isRequestEntity {
				return nil, mvcContextBindingError(fn.Name.Name, "request entity", name)
			}
			if _, isMultipartBody := multipartBodySelectors[name]; isMultipartBody {
				return nil, mvcContextBindingError(fn.Name.Name, "multipart body", name)
			}
			if _, isBound := paramBindings[name]; isBound {
				return nil, mvcContextBindingError(fn.Name.Name, "bound", name)
			}
			if contextSeen {
				return nil, mvcHandlerError(
					"must not declare multiple *arkarta/web.Context parameters", fn.Name.Name,
				)
			}
			contextSeen = true
			params = append(params, mvcHandlerParam{Name: name, Kind: mvcParamContext})
			continue
		}
		if isGoarkMVCModelPointerExpr(file, field.Type) {
			if _, isBody := bodySelectors[name]; isBody {
				return nil, mvcModelBindingError(fn.Name.Name, "request body", name)
			}
			if _, isRequestEntity := requestEntitySelectors[name]; isRequestEntity {
				return nil, mvcModelBindingError(fn.Name.Name, "request entity", name)
			}
			if _, isMultipartBody := multipartBodySelectors[name]; isMultipartBody {
				return nil, mvcModelBindingError(fn.Name.Name, "multipart body", name)
			}
			if _, isBound := paramBindings[name]; isBound {
				return nil, mvcHandlerError("bound parameter %s must not be *mvc.Model", fn.Name.Name, name)
			}
			if modelSeen {
				return nil, mvcHandlerError("must not declare multiple *mvc.Model parameters", fn.Name.Name)
			}
			modelSeen = true
			params = append(params, mvcHandlerParam{Name: name, Type: "mvc.Model", Kind: mvcParamModel})
			continue
		}
		if isSelectorTypeExpr(field.Type, "Context") {
			return nil, mvcHandlerError("parameter must be *arkarta/web.Context", fn.Name.Name)
		}
		requestEntityBodyType, isRequestEntityType := mvcRequestEntityBodyType(fset, file, field.Type)
		if _, isRequestEntity := requestEntitySelectors[name]; isRequestEntity || isRequestEntityType {
			if _, isBody := bodySelectors[name]; isBody {
				return nil, mvcMultipleBindingError(fn.Name.Name, name)
			}
			if _, isMultipartBody := multipartBodySelectors[name]; isMultipartBody {
				return nil, mvcMultipleBindingError(fn.Name.Name, name)
			}
			if _, isBound := paramBindings[name]; isBound {
				return nil, mvcMultipleBindingError(fn.Name.Name, name)
			}
			if !isRequestEntityType {
				return nil, mvcHandlerError(
					"request entity parameter %s must be goark.dev/goark/web.RequestEntity[T]",
					fn.Name.Name, name,
				)
			}
			if bodySeen {
				return nil, mvcHandlerError("must not declare multiple request body parameters", fn.Name.Name)
			}
			bodySeen = true
			params = append(params, mvcHandlerParam{
				Name: name, Type: "goweb.RequestEntity[" + requestEntityBodyType + "]",
				Kind: mvcParamRequestEntity, BodyType: requestEntityBodyType,
			})
			continue
		}
		if _, isBody := bodySelectors[name]; isBody {
			if _, isMultipartBody := multipartBodySelectors[name]; isMultipartBody {
				return nil, mvcMultipleBindingError(fn.Name.Name, name)
			}
			if _, isBound := paramBindings[name]; isBound {
				return nil, mvcMultipleBindingError(fn.Name.Name, name)
			}
			if bodySeen {
				return nil, mvcHandlerError("must not declare multiple request body parameters", fn.Name.Name)
			}
			bodySeen = true
			params = append(params, mvcHandlerParam{
				Name: name, Type: exprString(fset, field.Type), Kind: mvcParamBody,
			})
			continue
		}
		if _, isMultipartBody := multipartBodySelectors[name]; isMultipartBody {
			if _, isBound := paramBindings[name]; isBound {
				return nil, mvcMultipleBindingError(fn.Name.Name, name)
			}
			if bodySeen {
				return nil, mvcHandlerError("must not declare multiple request body parameters", fn.Name.Name)
			}
			bodySeen = true
			params = append(params, mvcHandlerParam{
				Name: name, Type: exprString(fset, field.Type), Kind: mvcParamMultipartBody,
			})
			continue
		}
		if binding, isBound := paramBindings[name]; isBound {
			typ := exprString(fset, field.Type)
			if binding.Kind == mvcParamModelAttribute && !isMVCModelAttributeTypeExpr(field.Type) {
				return nil, mvcHandlerError(
					"model attribute parameter %s must be a non-pointer struct value",
					fn.Name.Name, name,
				)
			}
			requestPartFile := binding.Kind == mvcParamRequestPart &&
				isArkartaMultipartPartExpr(file, field.Type)
			if err := validateMVCParameterMapBinding(
				fn.Name.Name, name, typ, binding.Kind, binding.Binding,
			); err != nil {
				return nil, err
			}
			param := mvcHandlerParam{
				Type: typ, Kind: binding.Kind,
				Binding: binding.Binding, RequestPartFile: requestPartFile,
			}
			if _, ok := mvcParameterBindingCall(param, nil); !ok {
				return nil, mvcHandlerError(
					"parameter %s has unsupported mvc parameter type %s",
					fn.Name.Name, name, typ,
				)
			}
			param.Name = name
			params = append(params, param)
			continue
		}
		return nil, mvcHandlerError(
			"parameter %s must be *arkarta/web.Context or annotated with mvc binding annotation",
			fn.Name.Name, name,
		)
	}
	for selector := range bodySelectors {
		if !mvcHasParam(params, selector) {
			return nil, mvcSelectorError(fn.Name.Name, "request body", selector)
		}
	}
	for selector := range requestEntitySelectors {
		if !mvcHasParam(params, selector) {
			return nil, mvcSelectorError(fn.Name.Name, "request entity", selector)
		}
	}
	for selector := range multipartBodySelectors {
		if !mvcHasParam(params, selector) {
			return nil, mvcSelectorError(fn.Name.Name, "multipart body", selector)
		}
	}
	for selector := range paramBindings {
		if !mvcHasParam(params, selector) {
			return nil, mvcSelectorError(fn.Name.Name, "parameter binding", selector)
		}
	}
	if bodySeen && hasMVCModelAttributeParam(params) {
		return nil, mvcHandlerError(
			"must not combine request body and model attribute parameters", fn.Name.Name,
		)
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
			return nil, fmt.Errorf(
				"mvc model attribute method %s parameter group must declare exactly one name",
				fn.Name.Name,
			)
		}
		name := names[0].Name
		if isArkWebContextExpr(file, field.Type) {
			if contextSeen {
				return nil, fmt.Errorf(
					"mvc model attribute method %s must not declare multiple "+
						"*arkarta/web.Context parameters",
					fn.Name.Name,
				)
			}
			contextSeen = true
			params = append(params, mvcHandlerParam{Name: name, Kind: mvcParamContext})
			continue
		}
		if isSelectorTypeExpr(field.Type, "Context") {
			return nil, fmt.Errorf(
				"mvc model attribute method %s parameter must be *arkarta/web.Context",
				fn.Name.Name,
			)
		}
		return nil, fmt.Errorf(
			"mvc model attribute method %s parameter %s must be *arkarta/web.Context",
			fn.Name.Name, name,
		)
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
	return 0, mvcHandlerError(
		"must return void, error, T, T,error, web.Result, web.Result,error, "+
			"web.ResponseEntity, web.ResponseEntity,error, web.DownloadResult, "+
			"or web.DownloadResult,error",
		fn.Name.Name,
	)
}

func mvcPrimaryReturnType(fset *token.FileSet, fn *ast.FuncDecl) string {
	if fn == nil || fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return ""
	}
	return exprString(fset, fn.Type.Results.List[0].Type)
}

func mvcPrimaryResponseEntityBodyType(
	fset *token.FileSet, file *ast.File, fn *ast.FuncDecl,
) string {
	if fn == nil || fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return ""
	}
	body, _ := mvcResponseEntityBodyType(fset, file, fn.Type.Results.List[0].Type)
	return body
}

func mvcValidationGroupArguments(groups []string) string {
	if len(groups) == 0 {
		return ""
	}
	var builder bytes.Buffer
	writeMVCValidationGroupArguments(&builder, groups)
	return builder.String()
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
