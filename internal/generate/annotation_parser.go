package generate

import (
	"go/ast"

	"goark.dev/cli/internal/generate/annotationparse"
)

// Annotation 表示一条 //goark:* 注解。
type Annotation = annotationparse.Annotation

// AnnotationArg 表示注解参数。
type AnnotationArg = annotationparse.AnnotationArg

func parseAnnotations(group *ast.CommentGroup) ([]Annotation, error) {
	return annotationparse.ParseComments(group)
}
