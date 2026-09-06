// Package mvcrouting 提供不依赖生成模型的 MVC 路由策略。
package mvcrouting

import (
	"net/http"
	"strings"
)

// SupportedMethod 判断 HTTP 方法是否可用于 MVC 路由。
func SupportedMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// DefaultStatus 返回路由的默认响应状态码。
func DefaultStatus(methods []string) int {
	if len(methods) == 1 && methods[0] == http.MethodPost {
		return http.StatusCreated
	}
	return http.StatusOK
}

// Constructor 返回 HTTP 方法对应的路由构造函数名。
func Constructor(method string) string {
	switch method {
	case http.MethodGet:
		return "GET"
	case http.MethodHead:
		return "HEAD"
	case http.MethodPost:
		return "POST"
	case http.MethodPut:
		return "PUT"
	case http.MethodPatch:
		return "PATCH"
	case http.MethodDelete:
		return "DELETE"
	case http.MethodOptions:
		return "OPTIONS"
	case http.MethodTrace:
		return "TRACE"
	default:
		return "Handle"
	}
}

// NormalizePath 将路由路径规范化为以斜杠开头且无尾斜杠的形式。
func NormalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(path, "/")
}

// NormalizePaths 按输入顺序规范化并去重路径。
func NormalizePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = NormalizePath(path)
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

// CombineMethods 将控制器方法补充到显式路由方法后并保持稳定顺序。
func CombineMethods(controllerMethods, routeMethods []string, routeMethodsSet bool) []string {
	if len(controllerMethods) == 0 {
		return routeMethods
	}
	if !routeMethodsSet {
		return append([]string(nil), controllerMethods...)
	}
	out := append([]string(nil), routeMethods...)
	seen := make(map[string]struct{}, len(routeMethods)+len(controllerMethods))
	for _, method := range routeMethods {
		seen[method] = struct{}{}
	}
	for _, method := range controllerMethods {
		if _, exists := seen[method]; exists {
			continue
		}
		seen[method] = struct{}{}
		out = append(out, method)
	}
	return out
}

// JoinPaths 合并控制器基础路径与方法路径。
func JoinPaths(base, path string) string {
	base = NormalizePath(base)
	path = NormalizePath(path)
	if base == "/" {
		return path
	}
	if path == "/" {
		return base
	}
	return base + path
}
