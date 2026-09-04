package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"goark.dev/cli/internal/generate"
)

type stringList []string

func (l *stringList) String() string {
	return strings.Join(*l, ",")
}

func (l *stringList) Set(value string) error {
	*l = append(*l, value)
	return nil
}

func (c Command) runCodegen(args []string) int {
	if len(args) == 0 {
		c.printCodegenHelp(c.Err)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		c.printCodegenHelp(c.Out)
		return 0
	case "configuration":
		return c.runCodegenConfiguration(args[1:])
	case "registry":
		return c.runCodegenRegistry(args[1:])
	case "annotations":
		return c.runCodegenAnnotations(args[1:])
	default:
		_, _ = fmt.Fprintf(c.Err, "未知生成器: %s\n\n", args[0])
		c.printCodegenHelp(c.Err)
		return 2
	}
}

func (c Command) runCodegenConfiguration(args []string) int {
	var imports stringList
	var beans stringList
	var output string
	spec := generate.ConfigurationSpec{}
	flags := flag.NewFlagSet("goark codegen configuration", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&spec.ConfigurationName, "name", "", "配置名称")
	flags.StringVar(&spec.PackageName, "package", "", "生成文件包名")
	flags.StringVar(&spec.TypeName, "type", "", "配置类型名，默认由 --name 推导")
	flags.IntVar(&spec.Order, "order", 0, "配置排序值")
	flags.StringVar(&output, "output", "", "输出文件路径，留空时输出到 stdout")
	flags.Var(&imports, "import", "额外导入，格式为 path 或 alias=path，可重复")
	flags.Var(&beans, "bean", "Bean 注册项，格式为 name=provider[;deps=a,b][;scope=prototype][;lazy][;primary]，可重复")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			c.printCodegenConfigurationHelp(c.Out)
			return 0
		}
		_, _ = fmt.Fprintf(c.Err, "%v\n\n", err)
		c.printCodegenConfigurationHelp(c.Err)
		return 2
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(c.Err, "多余参数: %s\n\n", strings.Join(flags.Args(), " "))
		c.printCodegenConfigurationHelp(c.Err)
		return 2
	}
	if spec.ConfigurationName == "" || spec.PackageName == "" {
		c.printCodegenConfigurationHelp(c.Err)
		return 2
	}
	for _, rawImport := range imports {
		item, err := parseImportSpec(rawImport)
		if err != nil {
			_, _ = fmt.Fprintf(c.Err, "%v\n\n", err)
			c.printCodegenConfigurationHelp(c.Err)
			return 2
		}
		spec.Imports = append(spec.Imports, item)
	}
	for _, rawBean := range beans {
		item, err := parseBeanSpec(rawBean)
		if err != nil {
			_, _ = fmt.Fprintf(c.Err, "%v\n\n", err)
			c.printCodegenConfigurationHelp(c.Err)
			return 2
		}
		spec.Beans = append(spec.Beans, item)
	}

	source, err := generate.GenerateConfiguration(spec)
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "%v\n", err)
		return 2
	}
	if output == "" {
		_, _ = c.Out.Write(source)
		return 0
	}
	if err := writeFile(output, source); err != nil {
		_, _ = fmt.Fprintf(c.Err, "写入生成文件失败: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(c.Err, "generated %s\n", output)
	return 0
}

func (c Command) runCodegenRegistry(args []string) int {
	var imports stringList
	var configurations stringList
	var output string
	spec := generate.RegistrySpec{}
	flags := flag.NewFlagSet("goark codegen registry", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&spec.PackageName, "package", "", "生成文件包名")
	flags.StringVar(&spec.FunctionName, "function", "", "注册函数名，默认 RegisterConfigurations")
	flags.StringVar(&output, "output", "", "输出文件路径，留空时输出到 stdout")
	flags.Var(&imports, "import", "额外导入，格式为 path 或 alias=path，可重复")
	flags.Var(&configurations, "configuration", "配置类型表达式，例如 AdminConfiguration 或 cfg.AdminConfiguration，可重复")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			c.printCodegenRegistryHelp(c.Out)
			return 0
		}
		_, _ = fmt.Fprintf(c.Err, "%v\n\n", err)
		c.printCodegenRegistryHelp(c.Err)
		return 2
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(c.Err, "多余参数: %s\n\n", strings.Join(flags.Args(), " "))
		c.printCodegenRegistryHelp(c.Err)
		return 2
	}
	if spec.PackageName == "" || len(configurations) == 0 {
		c.printCodegenRegistryHelp(c.Err)
		return 2
	}
	for _, rawImport := range imports {
		item, err := parseImportSpec(rawImport)
		if err != nil {
			_, _ = fmt.Fprintf(c.Err, "%v\n\n", err)
			c.printCodegenRegistryHelp(c.Err)
			return 2
		}
		spec.Imports = append(spec.Imports, item)
	}
	for _, rawConfiguration := range configurations {
		item := generate.ConfigurationRegistrationSpec{Type: strings.TrimSpace(rawConfiguration)}
		spec.Configurations = append(spec.Configurations, item)
	}

	source, err := generate.GenerateRegistry(spec)
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "%v\n", err)
		return 2
	}
	if output == "" {
		_, _ = c.Out.Write(source)
		return 0
	}
	if err := writeFile(output, source); err != nil {
		_, _ = fmt.Fprintf(c.Err, "写入生成文件失败: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(c.Err, "generated %s\n", output)
	return 0
}

func (c Command) runCodegenAnnotations(args []string) int {
	var output string
	spec := generate.AnnotationScanSpec{Dir: "."}
	flags := flag.NewFlagSet("goark codegen annotations", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&spec.Dir, "dir", ".", "待扫描 Go package 目录")
	flags.StringVar(&spec.PackageName, "package", "", "待扫描 package 名称，默认自动推导")
	flags.StringVar(&spec.ConfigurationName, "name", "", "无显式 configuration 时生成的配置名称")
	flags.StringVar(&spec.TypeName, "type", "", "无显式 configuration 时生成的配置类型名")
	flags.StringVar(&output, "output", "", "输出文件路径，留空时输出到 stdout")

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

	source, err := generate.GenerateAnnotations(spec)
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "%v\n", err)
		return 2
	}
	if output == "" {
		_, _ = c.Out.Write(source)
		return 0
	}
	if err := writeFile(output, source); err != nil {
		_, _ = fmt.Fprintf(c.Err, "写入生成文件失败: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(c.Err, "generated %s\n", output)
	return 0
}

func parseImportSpec(raw string) (generate.ImportSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return generate.ImportSpec{}, fmt.Errorf("import 不能为空")
	}
	alias, path, ok := strings.Cut(raw, "=")
	if !ok {
		return generate.ImportSpec{Path: raw}, nil
	}
	alias = strings.TrimSpace(alias)
	path = strings.TrimSpace(path)
	if alias == "" || path == "" {
		return generate.ImportSpec{}, fmt.Errorf("import %q 格式错误", raw)
	}
	return generate.ImportSpec{Alias: alias, Path: path}, nil
}

func parseBeanSpec(raw string) (generate.BeanSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return generate.BeanSpec{}, fmt.Errorf("bean 不能为空")
	}
	parts := strings.Split(raw, ";")
	name, provider, ok := strings.Cut(parts[0], "=")
	if !ok {
		return generate.BeanSpec{}, fmt.Errorf("bean %q 缺少 name=provider", raw)
	}
	bean := generate.BeanSpec{
		Name:     strings.TrimSpace(name),
		Provider: strings.TrimSpace(provider),
	}
	for _, option := range parts[1:] {
		option = strings.TrimSpace(option)
		if option == "" {
			continue
		}
		switch {
		case option == "lazy":
			bean.Lazy = true
		case option == "primary":
			bean.Primary = true
		case strings.HasPrefix(option, "deps="):
			deps := strings.TrimPrefix(option, "deps=")
			if deps == "" {
				return generate.BeanSpec{}, fmt.Errorf("bean %q deps 不能为空", bean.Name)
			}
			for _, dep := range strings.Split(deps, ",") {
				bean.Dependencies = append(bean.Dependencies, strings.TrimSpace(dep))
			}
		case strings.HasPrefix(option, "scope="):
			bean.Scope = strings.TrimSpace(strings.TrimPrefix(option, "scope="))
		default:
			return generate.BeanSpec{}, fmt.Errorf("bean %q 不支持选项 %q", bean.Name, option)
		}
	}
	return bean, nil
}

func writeFile(path string, data []byte) error {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c Command) printCodegenHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark codegen <generator> [flags]

Available generators:
  configuration     Generate a goark.Configuration source file.
  registry          Generate a Configuration registry source file.
  annotations       Scan //goark annotations and generate registration code.

`)
}

func (c Command) printCodegenConfigurationHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark codegen configuration --name <name> --package <package> [flags]

Flags:
  --name string       Required configuration name returned by Configuration.Name().
  --package string    Required generated Go package name.
  --type string       Configuration type name. Defaults to PascalCase(name) + Configuration.
  --order int         Configuration order. Defaults to 0.
  --output path       Output file path. Defaults to stdout.
  --import value      Extra import: path or alias=path. Repeatable.
  --bean value        Bean: name=provider[;deps=a,b][;scope=prototype][;lazy][;primary]. Repeatable.

Examples:
  goark codegen configuration --name user --package generated
  goark codegen configuration --name user --package generated --type UserConfiguration --order 100 --output internal/generated/user_configuration.go
  goark codegen configuration --name user --package generated --bean "userRepository=NewUserRepository" --bean "userService=NewUserService;deps=userRepository"

`)
}

func (c Command) printCodegenRegistryHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark codegen registry --package <package> --configuration <type> [flags]

Flags:
  --package string          Required generated Go package name.
  --function string         Registry function name. Defaults to RegisterConfigurations.
  --output path             Output file path. Defaults to stdout.
  --import value            Extra import: path or alias=path. Repeatable.
  --configuration value     Configuration type expression. Repeatable.

Examples:
  goark codegen registry --package generated --configuration UserConfiguration
  goark codegen registry --package generated --configuration UserConfiguration --configuration HTTPConfiguration --output internal/generated/registry.go
  goark codegen registry --package generated --import cfg=example.com/app/internal/config --configuration cfg.AdminConfiguration

`)
}

func (c Command) printCodegenAnnotationsHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark codegen annotations --dir <package-dir> [flags]

Flags:
  --dir path          Go package directory to scan. Defaults to current directory.
  --package string    Package name to scan when directory contains multiple packages.
  --name string       Generated configuration name when no //goark:configuration exists.
  --type string       Generated configuration type when no //goark:configuration exists.
  --output path       Output file path. Defaults to stdout.

Examples:
  goark codegen annotations --dir .
  goark codegen annotations --dir internal/app --output internal/app/zz_goark_app_gen.go

`)
}
