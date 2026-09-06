package generate

import (
	"go/ast"
	"strconv"
	"strings"
)

func isArkWebContextExpr(file *ast.File, expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	return isImportedSelectorExpr(file, star.X, arkartaWebImportPath, "Context")
}

func isGoarkMVCModelPointerExpr(file *ast.File, expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	return isImportedSelectorExpr(file, star.X, goarkMVCImportPath, "Model")
}

func isSelectorTypeExpr(expr ast.Expr, selectorName string) bool {
	star, ok := expr.(*ast.StarExpr)
	if ok {
		expr = star.X
	}
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == selectorName
}

func isErrorExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "error"
}

func isArkWebResultExpr(file *ast.File, expr ast.Expr) bool {
	return isImportedSelectorExpr(file, expr, arkartaWebImportPath, "Result")
}

func isGoarkWebResponseEntityExpr(file *ast.File, expr ast.Expr) bool {
	switch typ := expr.(type) {
	case *ast.IndexExpr:
		return isImportedSelectorExpr(file, typ.X, goarkWebImportPath, "ResponseEntity")
	case *ast.IndexListExpr:
		return isImportedSelectorExpr(file, typ.X, goarkWebImportPath, "ResponseEntity")
	default:
		return false
	}
}

func isGoarkWebDownloadResultExpr(file *ast.File, expr ast.Expr) bool {
	return isImportedSelectorExpr(file, expr, goarkWebImportPath, "DownloadResult")
}

func isImportedSelectorExpr(file *ast.File, expr ast.Expr, importPath string, selectorName string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != selectorName {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	aliases := importAliases(file, importPath)
	_, ok = aliases[ident.Name]
	return ok
}

func importAliases(file *ast.File, importPath string) map[string]struct{} {
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
	index := strings.LastIndex(importPath, "/")
	if index < 0 {
		return importPath
	}
	return importPath[index+1:]
}

func mvcModelUsesOptionalInjection(model *mvcAnnotationModel) bool {
	for _, controller := range model.Controllers {
		for _, field := range controller.Component.Fields {
			if !field.Injection.Required && field.Injection.Kind != "value" {
				return true
			}
		}
	}
	for _, advice := range model.Advices {
		for _, field := range advice.Component.Fields {
			if !field.Injection.Required && field.Injection.Kind != "value" {
				return true
			}
		}
	}
	return false
}

func mvcModelUsesConfigurer(model *mvcAnnotationModel) bool {
	if len(model.Controllers) > 0 {
		return true
	}
	for _, advice := range model.Advices {
		if len(advice.ExceptionHandlers) > 0 {
			return true
		}
	}
	return false
}

func mvcModelUsesArkWeb(model *mvcAnnotationModel) bool {
	for _, controller := range model.Controllers {
		if len(controller.Routes) > 0 || len(controller.ModelAttributes) > 0 {
			return true
		}
	}
	for _, advice := range model.Advices {
		if len(advice.ExceptionHandlers) > 0 {
			return true
		}
	}
	return false
}
