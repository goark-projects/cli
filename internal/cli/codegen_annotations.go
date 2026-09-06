package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/generate"
	"goark.dev/cli/internal/runargs"
)

func (c Command) runCodegenAnnotations(args []string) int {
	spec := generate.AnnotationScanSpec{Dir: "."}
	flags := flag.NewFlagSet("goark codegen annotations", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&spec.Dir, "dir", ".", "待扫描 Go package 目录")
	flags.StringVar(&spec.PackageName, "package", "", "待扫描 package 名称")
	flags.StringVar(&spec.ConfigurationName, "name", "", "默认配置名称")
	flags.StringVar(&spec.TypeName, "type", "", "默认配置类型名")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			c.printCodegenAnnotationsHelp(c.Out)
			return 0
		}
		_, _ = fmt.Fprintf(c.Err, "%v\n\n", err)
		c.printCodegenAnnotationsHelp(c.Err)
		return 2
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(c.Err, "多余参数: %s\n\n", strings.Join(flags.Args(), " "))
		c.printCodegenAnnotationsHelp(c.Err)
		return 2
	}
	results, err := c.generateAnnotationPackage(spec)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	for _, result := range results {
		if result.Removed {
			_, _ = fmt.Fprintf(c.Err, "removed %s\n", result.Output)
			continue
		}
		_, _ = fmt.Fprintf(c.Err, "generated %s\n", result.Output)
	}
	return 0
}

func (c Command) generateAnnotationPackage(
	spec generate.AnnotationScanSpec,
) ([]GenerationResult, error) {
	directory := spec.Dir
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(runargs.BaseDir(c.Dir), directory)
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("解析 package 目录失败: %w", err)
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, fmt.Errorf("解析 package 目录符号链接失败: %w", err)
	}
	resolver := projectResolver{Dir: directory, Env: c.environment(), Static: true}
	module, err := resolver.resolveModuleStatic()
	if err != nil {
		return nil, err
	}
	pattern, err := modulePackagePattern(module.Dir, directory)
	if err != nil {
		return nil, err
	}
	packages, err := resolver.listPackagesStatic(module.Dir, module.Path, []string{pattern})
	if err != nil {
		return nil, err
	}
	if len(packages) != 1 || packages[0].Dir != directory {
		return nil, fmt.Errorf("目录 %s 不包含可生成的 Go package", directory)
	}
	if spec.PackageName != "" && packages[0].Name != spec.PackageName {
		return nil, fmt.Errorf("目录 %s 不包含 package %q", directory, spec.PackageName)
	}
	project := goarkProject{
		Root:       module.Dir,
		ModulePath: module.Path,
		Packages:   packages,
		Build: buildspec.Document{
			Generate: buildspec.Generate{CleanStale: true},
		},
	}
	generator := annotationProjectGenerator{
		configurationName: spec.ConfigurationName,
		typeName:          spec.TypeName,
	}
	return generator.Generate(project, false)
}

func modulePackagePattern(root string, directory string) (string, error) {
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("package 目录必须位于当前 Go 模块内: %s", directory)
	}
	if relative == "." {
		return ".", nil
	}
	return "./" + filepath.ToSlash(relative), nil
}
