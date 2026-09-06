package generate

import (
	"bytes"
	"strconv"
	"strings"
)

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
