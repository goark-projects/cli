package generate

import (
	"bytes"
	"strconv"
	"strings"
)

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
