package generate

import (
	"bytes"
	"strconv"
	"strings"
)

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
