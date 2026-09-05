// Package version 统一解析 Goark CLI 的构建版本。
package version

import (
	"runtime/debug"
	"strings"
)

const Development = "devel"

// Build 可由发布构建通过 -ldflags 注入。
var Build string

// Current 返回发布构建版本、Go 模块版本或开发版本。
func Current() string {
	moduleVersion := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = info.Main.Version
	}
	return resolve(Build, moduleVersion)
}

func resolve(buildVersion string, moduleVersion string) string {
	if version := normalize(buildVersion); version != "" {
		return version
	}
	if version := normalize(moduleVersion); version != "" && version != "(devel)" {
		return version
	}
	return Development
}

func normalize(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}
