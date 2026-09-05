// Package envutil 提供跨平台一致的环境变量名称语义。
package envutil

import (
	"runtime"
	"sort"
	"strings"
)

// Set 按当前平台的环境名规则设置一个值。
func Set(environment map[string]string, name string, value string) {
	if runtime.GOOS == "windows" {
		for existing := range environment {
			if existing != name && strings.EqualFold(existing, name) {
				delete(environment, existing)
			}
		}
	}
	environment[name] = value
}

// Overlay 以稳定顺序将一层环境覆盖到目标环境。
func Overlay(target map[string]string, source map[string]string) {
	names := make([]string, 0, len(source))
	for name := range source {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		Set(target, name, source[name])
	}
}

// Lookup 按当前平台的环境名规则读取一个值。
func Lookup(environment map[string]string, name string) (string, bool) {
	value, ok := environment[name]
	if ok || runtime.GOOS != "windows" {
		return value, ok
	}
	for candidate, candidateValue := range environment {
		if strings.EqualFold(candidate, name) {
			return candidateValue, true
		}
	}
	return "", false
}
