package goargs

import (
	"fmt"
	"strings"
)

// ParseFlags 按 Go 的 GOFLAGS 规则解析整项引号，不执行转义或变量展开。
func ParseFlags(value string) ([]string, error) {
	var arguments []string
	for {
		value = strings.TrimLeft(value, " \t\r\n")
		if value == "" {
			return arguments, nil
		}
		if quote := value[0]; quote == '\'' || quote == '"' {
			end := strings.IndexByte(value[1:], quote)
			if end < 0 {
				return nil, fmt.Errorf("GOFLAGS 引号未闭合")
			}
			arguments = append(arguments, value[1:end+1])
			value = value[end+2:]
			continue
		}
		end := strings.IndexAny(value, " \t\r\n")
		if end < 0 {
			return append(arguments, value), nil
		}
		arguments = append(arguments, value[:end])
		value = value[end:]
	}
}
