package generate

import (
	"fmt"
	"strings"
)

func mvcParameterMapFunction(kind mvcHandlerParamKind, typ string) (string, bool) {
	switch strings.TrimSpace(typ) {
	case "map[string]string":
		switch kind {
		case mvcParamRequestParam:
			return "RequestParamMap", true
		case mvcParamRequestHeader:
			return "RequestHeaderMap", true
		default:
			return "", false
		}
	case "map[string][]string":
		switch kind {
		case mvcParamRequestParam:
			return "RequestParamValuesMap", true
		case mvcParamRequestHeader:
			return "RequestHeaderValuesMap", true
		default:
			return "", false
		}
	default:
		return "", false
	}
}

func validateMVCParameterMapBinding(methodName, paramName, typ string, kind mvcHandlerParamKind, binding mvcParamBinding) error {
	if _, ok := mvcParameterMapFunction(kind, typ); !ok {
		return nil
	}
	if binding.SourceExplicit || binding.HasDefault || !binding.Required {
		return fmt.Errorf("mvc handler method %s map parameter %s must not declare name, value, defaultValue, or required=false", methodName, paramName)
	}
	return nil
}
