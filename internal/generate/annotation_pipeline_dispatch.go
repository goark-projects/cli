package generate

import (
	"fmt"
	"go/ast"
	"go/token"
)

func scanAnnotationFile(ctx *AnnotationBindingContext, pipeline *annotationPipeline, fset *token.FileSet, file *ast.File) error {
	for _, decl := range file.Decls {
		switch item := decl.(type) {
		case *ast.GenDecl:
			if item.Tok == token.TYPE {
				if err := scanTypeDeclaration(ctx, pipeline, fset, file, item); err != nil {
					return err
				}
			}
		case *ast.FuncDecl:
			annotations, err := parseAnnotations(item.Doc)
			if err != nil {
				return err
			}
			if len(annotations) == 0 {
				continue
			}
			annotationItem := AnnotationItem{
				target:      AnnotationTargetMethod,
				packageName: ctx.PackageName(),
				fset:        fset,
				file:        file,
				funcDecl:    item,
				annotations: annotations,
			}
			if err := pipeline.dispatch(ctx, annotationItem); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanTypeDeclaration(ctx *AnnotationBindingContext, pipeline *annotationPipeline, fset *token.FileSet, file *ast.File, decl *ast.GenDecl) error {
	typeAnnotations, err := parseAnnotations(decl.Doc)
	if err != nil {
		return err
	}
	for _, spec := range decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		specAnnotations, err := parseAnnotations(typeSpec.Doc)
		if err != nil {
			return err
		}
		annotations := mergeAnnotations(typeAnnotations, specAnnotations)
		if len(annotations) > 0 {
			item := AnnotationItem{
				target:      AnnotationTargetType,
				packageName: ctx.PackageName(),
				fset:        fset,
				file:        file,
				genDecl:     decl,
				typeSpec:    typeSpec,
				annotations: annotations,
			}
			if err := pipeline.dispatch(ctx, item); err != nil {
				return err
			}
		}
		structType, ok := typeSpec.Type.(*ast.StructType)
		if ok {
			if err := scanStructFields(ctx, pipeline, fset, file, decl, typeSpec, structType); err != nil {
				return err
			}
		}
		interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
		if ok {
			if err := scanInterfaceMethods(ctx, pipeline, fset, file, decl, typeSpec, interfaceType); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanStructFields(ctx *AnnotationBindingContext, pipeline *annotationPipeline, fset *token.FileSet, file *ast.File, decl *ast.GenDecl, typeSpec *ast.TypeSpec, structType *ast.StructType) error {
	for _, field := range structType.Fields.List {
		fieldAnnotations, err := parseAnnotations(field.Doc)
		if err != nil {
			return err
		}
		if len(fieldAnnotations) == 0 {
			continue
		}
		item := AnnotationItem{
			target:      AnnotationTargetField,
			packageName: ctx.PackageName(),
			fset:        fset,
			file:        file,
			genDecl:     decl,
			typeSpec:    typeSpec,
			field:       field,
			annotations: fieldAnnotations,
		}
		if err := pipeline.dispatch(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func scanInterfaceMethods(ctx *AnnotationBindingContext, pipeline *annotationPipeline, fset *token.FileSet, file *ast.File, decl *ast.GenDecl, typeSpec *ast.TypeSpec, interfaceType *ast.InterfaceType) error {
	for _, method := range interfaceType.Methods.List {
		methodAnnotations, err := parseAnnotations(method.Doc)
		if err != nil {
			return err
		}
		if len(methodAnnotations) == 0 {
			continue
		}
		item := AnnotationItem{
			target:      AnnotationTargetMethod,
			packageName: ctx.PackageName(),
			fset:        fset,
			file:        file,
			genDecl:     decl,
			typeSpec:    typeSpec,
			field:       method,
			annotations: methodAnnotations,
		}
		if err := pipeline.dispatch(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func mergeAnnotations(left []Annotation, right []Annotation) []Annotation {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	out := make([]Annotation, 0, len(left)+len(right))
	out = append(out, left...)
	out = append(out, right...)
	return out
}

func (p *annotationPipeline) dispatch(ctx *AnnotationBindingContext, item AnnotationItem) error {
	if err := p.validate(item); err != nil {
		return err
	}
	for _, extension := range p.extensions {
		if extension.Binder == nil || !itemMatchesDescriptors(item, extension.Descriptors) {
			continue
		}
		if err := extension.Binder.BindAnnotation(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (p *annotationPipeline) validate(item AnnotationItem) error {
	for _, annotation := range item.annotations {
		descriptor, ok := p.descriptors[annotation.Name]
		if !ok {
			return fmt.Errorf("unknown annotation %q", annotation.Name)
		}
		if !descriptorAllowsTarget(descriptor, item.Target()) {
			return fmt.Errorf("annotation %q does not support %s target", annotation.Name, item.Target())
		}
		if descriptor.Validate == nil {
			continue
		}
		ctx := AnnotationValidationContext{
			Target:     item.Target(),
			Annotation: annotation,
			Item:       item,
		}
		if err := descriptor.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}

func descriptorAllowsTarget(descriptor AnnotationDescriptor, target AnnotationTarget) bool {
	if len(descriptor.Targets) == 0 {
		return true
	}
	for _, item := range descriptor.Targets {
		if item == target {
			return true
		}
	}
	return false
}

func itemMatchesDescriptors(item AnnotationItem, descriptors []AnnotationDescriptor) bool {
	for _, descriptor := range descriptors {
		if item.HasAnnotation(descriptor.Name) {
			return true
		}
	}
	return false
}
