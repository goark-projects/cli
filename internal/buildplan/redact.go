package buildplan

import (
	"sort"
	"strings"
)

// RedactArguments 返回保留参数结构但隐藏密钥值的副本。
func RedactArguments(arguments []string, environment map[string]string) []string {
	secretValues := make([]string, 0)
	for name, value := range environment {
		if value != "" && isSecretName(name) {
			secretValues = append(secretValues, value)
		}
	}
	sort.Slice(secretValues, func(i, j int) bool { return len(secretValues[i]) > len(secretValues[j]) })
	result := make([]string, len(arguments))
	for index, argument := range arguments {
		argument = redactAssignment(argument)
		for _, value := range secretValues {
			argument = strings.ReplaceAll(argument, value, RedactedValue)
		}
		result[index] = argument
	}
	return result
}

func redactAssignment(argument string) string {
	name, _, ok := strings.Cut(argument, "=")
	if !ok || !isSecretName(strings.TrimLeft(name, "-D/")) {
		return argument
	}
	return name + "=" + RedactedValue
}
