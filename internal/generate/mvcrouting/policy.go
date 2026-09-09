// Package mvcrouting 兼容原有生成器内部调用，路由语义由公共 MVC 契约提供。
package mvcrouting

import routing "goark.dev/goark/web/mvc/routing"

// SupportedMethod 判断受支持的方法。
func SupportedMethod(method string) bool { return routing.SupportedMethod(method) }

// DefaultStatus 返回标准默认状态。
func DefaultStatus(methods []string) int { return routing.DefaultStatus(methods) }

// Constructor 返回 MVC 构造函数名称。
func Constructor(method string) string { return routing.Constructor(method) }

// NormalizePath 规范化路径。
func NormalizePath(path string) string { return routing.NormalizePath(path) }

// NormalizePaths 规范化并去重路径。
func NormalizePaths(paths []string) []string { return routing.NormalizePaths(paths) }

// CombineMethods 合并控制器与方法声明。
func CombineMethods(controller, methods []string, explicit bool) []string {
	return routing.CombineMethods(controller, methods, explicit)
}

// JoinPaths 合并基础路径与方法路径。
func JoinPaths(base, path string) string { return routing.JoinPaths(base, path) }
