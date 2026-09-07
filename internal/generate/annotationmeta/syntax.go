package annotationmeta

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
)

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
