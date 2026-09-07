package annotationmeta

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Int 返回同名注解的整数 value，非法值使用回退值。
func Int(annotations []Annotation, name string, fallback int) int {
	for _, annotation := range annotations {
		if annotation.Name != name {
			continue
		}
		if value, ok := annotation.Args["value"]; ok {
			if parsed, err := strconv.Atoi(value.Text()); err == nil {
				return parsed
			}
		}
	}
	return fallback
}

// Bool 返回注解布尔参数，缺失或非法时使用回退值。
func Bool(annotation Annotation, key string, fallback bool) bool {
	value, ok := annotation.Args[key]
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(value.Text())
	if err != nil {
		return fallback
	}
	return parsed
}

// BoolByName 返回同名注解的布尔 value。
func BoolByName(annotations []Annotation, name string, fallback bool) bool {
	for _, annotation := range annotations {
		if annotation.Name == name {
			return Bool(annotation, "value", fallback)
		}
	}
	return fallback
}

// ArgString 返回指定字符串参数。
func ArgString(annotation Annotation, key, fallback string) string {
	if value, ok := annotation.Args[key]; ok {
		return value.Text()
	}
	return fallback
}

// FirstNonEmpty 返回首个非空字符串。
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// LowerCamel 将标识符首个字符转换为小写。
func LowerCamel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	r, size := utf8.DecodeRuneInString(value)
	if r == utf8.RuneError {
		return value
	}
	return string(unicode.ToLower(r)) + value[size:]
}
