package application_test

import (
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
	"goark.dev/cli/internal/generate/application"
)

func TestDuplicateApplicationAnnotation(t *testing.T) {
	dir := writeSource(t, `package app
//goark:application
//goark:application(web=true)
//goark:configuration("app")
type Application struct{}
`)
	_, err := generate.GenerateAnnotationFiles(generate.AnnotationScanSpec{
		Dir: dir, Extensions: []generate.AnnotationExtension{application.Extension()},
	})
	if err == nil || !strings.Contains(err.Error(), "multiple application") {
		t.Fatalf("重复应用注解未被拒绝: %v", err)
	}
}
