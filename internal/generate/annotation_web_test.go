package generate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestGenerateAnnotations_whenWebInterceptorAndFilterExist_shouldGenerateWebConfiguration(t *testing.T) {
	dir := t.TempDir()
	source := `package app

import (
	"context"

	"goark.dev/arkarta/servlet"
	arkweb "goark.dev/arkarta/web"
)

//goark:web-interceptor("traceInterceptor")
type TraceInterceptor struct{}

func (i *TraceInterceptor) Intercept(ctx *arkweb.Context, next arkweb.Handler) (arkweb.Result, error) {
	return next.Handle(ctx)
}

//goark:web-filter("auditFilter")
type AuditFilter struct{}

func (f *AuditFilter) Filter(ctx context.Context, req *servlet.Request, res servlet.Response, chain servlet.Chain) error {
	return chain.Next(ctx, req, res)
}
`
	if err := os.WriteFile(filepath.Join(dir, "web.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	generated, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate annotations failed: %v", err)
	}
	assertGeneratedPackageBuilds(t, dir, generated)
	text := string(generated)
	expected := []string{
		"type GoarkWebConfiguration struct{}",
		"container.Register(registry, \"auditFilter\"",
		"container.Register(registry, \"traceInterceptor\"",
		"container.Register[goweb.Configurer](registry, \"auditFilter.webFilterConfigurer\"",
		"container.Register[goweb.Configurer](registry, \"traceInterceptor.webInterceptorConfigurer\"",
		"container.GetByType[*AuditFilter](ctx, resolver, container.WithQualifier(\"auditFilter\"))",
		"container.GetByType[*TraceInterceptor](ctx, resolver, container.WithQualifier(\"traceInterceptor\"))",
		"webRegistry.AddFilter(filter)",
		"webRegistry.Use(interceptor)",
		"container.WithFactoryDependencies(\"auditFilter\")",
		"container.WithFactoryDependencies(\"traceInterceptor\")",
	}
	for _, fragment := range expected {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated web source missing %q:\n%s", fragment, text)
		}
	}
}

func TestGenerateAnnotations_whenWebAnnotationMisplaced_shouldReturnValidationError(t *testing.T) {
	dir := t.TempDir()
	source := `package app

type TraceInterceptor struct{}

//goark:web-interceptor
func (i *TraceInterceptor) Install() {}
`
	if err := os.WriteFile(filepath.Join(dir, "web.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source failed: %v", err)
	}

	_, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), `annotation "web-interceptor" does not support method target`) {
		t.Fatalf("expected web annotation target error, got %v", err)
	}
}
