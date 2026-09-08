package annotationparse

import (
	"fmt"
	"go/ast"
	"slices"
	"strings"
)

// Namespaces 保存当前生成器接管的命名空间，隔离其他生成器的语法。
type Namespaces []string

// Register 根据描述符名称注册领域；核心描述符使用不带前缀的名称。
func (n *Namespaces) Register(name string) error {
	namespace, local, qualified := strings.Cut(name, ":")
	if !qualified {
		return nil
	}
	invalidLocal := local == "" || strings.ContainsAny(local, ": \t\r\n([")
	if namespace == "goark" || invalidLocal || !IsAnnotation(name) {
		return fmt.Errorf("invalid annotation descriptor name %q", name)
	}
	if !slices.Contains(*n, namespace) {
		*n = append(*n, namespace)
	}
	return nil
}

// ParseComments 只解析当前生成器接管的领域。
func (n Namespaces) ParseComments(group *ast.CommentGroup) ([]Annotation, error) {
	return ParseComments(group, n...)
}

// UnknownAnnotation 返回未知注解错误，或旧 Web 注解的迁移提示。
func UnknownAnnotation(name string, migratedWeb bool) error {
	if migratedWeb {
		return fmt.Errorf("annotation //goark:%s moved to //goark-web:%s", name, name)
	}
	return fmt.Errorf("unknown annotation %q", name)
}
