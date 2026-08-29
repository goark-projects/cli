package generate

import (
	"bytes"
	"strconv"
	"strings"
)

func mvcRequestPartBindingCall(param mvcHandlerParam, validationGroups []string) (string, bool) {
	args := []string{"ctx", strconv.Quote(param.Binding.SourceName)}
	if !param.Binding.Required {
		args = append(args, "mvc.WithRequired(false)")
	}
	if param.RequestPartFile {
		return "mvc.RequestPart(" + strings.Join(args, ", ") + ")", true
	}
	if len(validationGroups) == 0 {
		return "mvc.RequestPartJSON[" + param.Type + "](" + strings.Join(args, ", ") + ")", true
	}
	return "mvc.ValidatedRequestPartJSON[" + param.Type + "](" + strings.Join(mvcValidatedRequestPartArgs(param, validationGroups), ", ") + ")", true
}

func mvcValidatedRequestPartArgs(param mvcHandlerParam, validationGroups []string) []string {
	args := []string{"ctx", strconv.Quote(param.Binding.SourceName)}
	var groups bytes.Buffer
	writeMVCValidationGroupSlice(&groups, validationGroups)
	args = append(args, groups.String())
	if !param.Binding.Required {
		args = append(args, "mvc.WithRequired(false)")
	}
	return args
}

func hasMVCJSONRequestPartParam(params []mvcHandlerParam) bool {
	for _, param := range params {
		if param.Kind == mvcParamRequestPart && !param.RequestPartFile {
			return true
		}
	}
	return false
}
