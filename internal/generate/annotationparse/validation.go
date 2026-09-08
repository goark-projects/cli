package annotationparse

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// ParseGroups 合并声明前与行尾注解，保留重复项供语义校验。
func (n Namespaces) ParseGroups(groups ...*ast.CommentGroup) ([]Annotation, error) {
	var out []Annotation
	for _, group := range groups {
		items, err := n.ParseComments(group)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

// ValidatePlacement 拒绝当前命名空间内未绑定到扫描目标的注解。
func (n Namespaces) ValidatePlacement(file *ast.File, fset *token.FileSet) error {
	allowed := make(map[*ast.CommentGroup]bool)
	add := func(groups ...*ast.CommentGroup) {
		for _, group := range groups {
			allowed[group] = true
		}
	}
	for _, decl := range file.Decls {
		switch value := decl.(type) {
		case *ast.FuncDecl:
			add(value.Doc)
		case *ast.GenDecl:
			if value.Tok != token.TYPE {
				continue
			}
			add(value.Doc)
			for _, raw := range value.Specs {
				typ := raw.(*ast.TypeSpec)
				add(typ.Doc, typ.Comment)
				var fields *ast.FieldList
				switch body := typ.Type.(type) {
				case *ast.StructType:
					fields = body.Fields
				case *ast.InterfaceType:
					fields = body.Methods
				}
				if fields != nil {
					for _, field := range fields.List {
						add(field.Doc, field.Comment)
					}
				}
			}
		}
	}
	for _, group := range file.Comments {
		if allowed[group] {
			continue
		}
		items, err := n.ParseComments(group)
		if err != nil {
			return err
		}
		if len(items) > 0 {
			return fmt.Errorf("%s: annotation %q has unsupported or unattached target",
				fset.Position(group.Pos()), items[0].Name)
		}
	}
	return nil
}

// TargetError 为注解错误补充声明位置及完整字段或方法名。
func TargetError(fset *token.FileSet, typ *ast.TypeSpec, field *ast.Field,
	fn *ast.FuncDecl, receiver string, err error,
) error {
	pos, owner := token.NoPos, receiver
	if typ != nil {
		pos, owner = typ.Pos(), typ.Name.Name
	}
	if field != nil {
		pos = field.Pos()
		var names []string
		for _, name := range field.Names {
			names = append(names, owner+"."+name.Name)
		}
		owner = strings.Join(names, ", ")
	}
	if fn != nil {
		pos, owner = fn.Pos(), strings.Trim(owner+"."+fn.Name.Name, ".")
	}
	return fmt.Errorf("%s: %s: %w", fset.Position(pos), owner, err)
}
