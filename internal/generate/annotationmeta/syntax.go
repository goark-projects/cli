package annotationmeta

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"strconv"
	"strings"
)

// ImportedSelector 判断表达式是否来自指定导入路径和选择器。
func ImportedSelector(
	file *ast.File,
	expression ast.Expr,
	importPath string,
	selectorName string,
) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != selectorName {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = ImportAliases(file, importPath)[identifier.Name]
	return ok
}

// ImportAliases 返回指定导入路径在文件中的有效别名。
func ImportAliases(file *ast.File, importPath string) map[string]struct{} {
	aliases := make(map[string]struct{}, 1)
	if file == nil {
		return aliases
	}
	for _, spec := range file.Imports {
		if spec.Path == nil {
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != importPath {
			continue
		}
		if spec.Name == nil {
			aliases[defaultImportName(importPath)] = struct{}{}
			continue
		}
		switch spec.Name.Name {
		case "", "_", ".":
			continue
		default:
			aliases[spec.Name.Name] = struct{}{}
		}
	}
	return aliases
}

func defaultImportName(importPath string) string {
	importPath = strings.Trim(importPath, "/")
	if importPath == "" {
		return ""
	}
	if index := strings.LastIndex(importPath, "/"); index >= 0 {
		return importPath[index+1:]
	}
	return importPath
}

// ReceiverTypeName 返回方法接收者的基础类型名。
func ReceiverTypeName(receiver *ast.FieldList) string {
	if receiver == nil || len(receiver.List) == 0 {
		return ""
	}
	switch typ := receiver.List[0].Type.(type) {
	case *ast.Ident:
		return typ.Name
	case *ast.StarExpr:
		if ident, ok := typ.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// ExprString 返回稳定的 Go 表达式文本。
func ExprString(fileSet *token.FileSet, expression ast.Expr) string {
	var builder bytes.Buffer
	_ = printer.Fprint(&builder, fileSet, expression)
	return builder.String()
}

// WrapExpressions 为每个条件表达式添加括号。
func WrapExpressions(expressions []string) []string {
	out := make([]string, 0, len(expressions))
	for _, expression := range expressions {
		out = append(out, "("+expression+")")
	}
	return out
}
