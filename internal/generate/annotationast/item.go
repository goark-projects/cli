package annotationast

import (
	"go/ast"
	"go/token"
	"goark.dev/cli/internal/generate/annotationmeta"
	"goark.dev/cli/internal/generate/annotationparse"
)

// Target 表示注解所在的 Go 语法目标。
type Target string

const (
	// TargetType 表示类型声明注解。
	TargetType Target = "type"
	// TargetField 表示结构体字段注解。
	TargetField Target = "field"
	// TargetMethod 表示函数或方法注解。
	TargetMethod Target = "method"
)

// Item 表示扫描器发现的一处带 goark 注解的语法节点。
type Item struct{ node Info }

// Info 保存扫描阶段采集的语法信息。
type Info struct {
	Target      Target
	PackageName string
	FileSet     *token.FileSet
	File        *ast.File
	GenDecl     *ast.GenDecl
	TypeSpec    *ast.TypeSpec
	Field       *ast.Field
	FuncDecl    *ast.FuncDecl
	Annotations []annotationparse.Annotation
}

// NewItem 根据扫描结果构造注解节点。
func NewItem(info Info) Item { return Item{node: info} }

// Target 返回当前注解所在语法目标。
func (i Item) Target() Target { return i.node.Target }

// PackageName 返回当前扫描包名。
func (i Item) PackageName() string { return i.node.PackageName }

// FileSet 返回当前扫描文件集。
func (i Item) FileSet() *token.FileSet { return i.node.FileSet }

// File 返回当前 AST 文件。
func (i Item) File() *ast.File { return i.node.File }

// GenDecl 返回当前通用声明，仅类型目标有效。
func (i Item) GenDecl() *ast.GenDecl { return i.node.GenDecl }

// TypeSpec 返回当前类型声明，仅类型或字段目标有效。
func (i Item) TypeSpec() *ast.TypeSpec { return i.node.TypeSpec }

// Field 返回当前字段声明，仅字段目标有效。
func (i Item) Field() *ast.Field { return i.node.Field }

// FuncDecl 返回当前函数声明，仅方法目标有效。
func (i Item) FuncDecl() *ast.FuncDecl { return i.node.FuncDecl }

// TypeName 返回当前类型名。
func (i Item) TypeName() string {
	if i.node.TypeSpec == nil {
		return ""
	}
	return i.node.TypeSpec.Name.Name
}

// FuncName 返回当前函数名。
func (i Item) FuncName() string {
	if i.node.FuncDecl == nil {
		return ""
	}
	return i.node.FuncDecl.Name.Name
}

// ReceiverTypeName 返回方法接收者类型名。
func (i Item) ReceiverTypeName() string {
	if i.node.FuncDecl == nil {
		return ""
	}
	return annotationmeta.ReceiverTypeName(i.node.FuncDecl.Recv)
}

// FieldNames 返回当前字段名列表。
func (i Item) FieldNames() []string {
	if i.Target() != TargetField {
		return nil
	}
	return i.Names()
}

// Names 返回当前语法目标声明的名称列表。
func (i Item) Names() []string {
	if i.node.FuncDecl != nil {
		return []string{i.node.FuncDecl.Name.Name}
	}
	if i.node.Field == nil {
		return nil
	}
	names := make([]string, 0, len(i.node.Field.Names))
	for _, name := range i.node.Field.Names {
		names = append(names, name.Name)
	}
	return names
}

// Annotations 返回当前节点上的 goark 注解副本。
func (i Item) Annotations() []annotationparse.Annotation {
	out := make([]annotationparse.Annotation, len(i.node.Annotations))
	copy(out, i.node.Annotations)
	return out
}

// HasAnnotation 判断当前节点是否存在指定注解。
func (i Item) HasAnnotation(key string) bool { return annotationmeta.Has(i.node.Annotations, key) }
