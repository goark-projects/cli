package codegencmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"goark.dev/cli/internal/generate/manual"
)

type stringList []string

func (l *stringList) String() string {
	return strings.Join(*l, ",")
}

func (l *stringList) Set(value string) error {
	*l = append(*l, value)
	return nil
}

// Run 分发低级代码生成器。
func Run(args []string, out io.Writer, errOut io.Writer, annotations func([]string) int) int {
	if len(args) == 0 {
		Help(errOut)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		Help(out)
		return 0
	case "configuration":
		return runConfiguration(args[1:], out, errOut)
	case "registry":
		return runRegistry(args[1:], out, errOut)
	case "annotations":
		return annotations(args[1:])
	default:
		_, _ = fmt.Fprintf(errOut, "未知生成器: %s\n\n", args[0])
		Help(errOut)
		return 2
	}
}

func runConfiguration(args []string, out io.Writer, errOut io.Writer) int {
	var imports stringList
	var beans stringList
	var output string
	spec := manual.ConfigurationSpec{}
	flags := flag.NewFlagSet("goark codegen configuration", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&spec.ConfigurationName, "name", "", "配置名称")
	flags.StringVar(&spec.PackageName, "package", "", "生成文件包名")
	flags.StringVar(&spec.TypeName, "type", "", "配置类型名，默认由 --name 推导")
	flags.IntVar(&spec.Order, "order", 0, "配置排序值")
	flags.StringVar(&output, "output", "", "输出文件路径，留空时输出到 stdout")
	flags.Var(&imports, "import", "额外导入，格式为 path 或 alias=path，可重复")
	flags.Var(
		&beans,
		"bean",
		"Bean 注册项，格式为 name=provider[;deps=a,b][;scope=prototype][;lazy][;primary]，可重复",
	)

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			ConfigurationHelp(out)
			return 0
		}
		_, _ = fmt.Fprintf(errOut, "%v\n\n", err)
		ConfigurationHelp(errOut)
		return 2
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(errOut, "多余参数: %s\n\n", strings.Join(flags.Args(), " "))
		ConfigurationHelp(errOut)
		return 2
	}
	if spec.ConfigurationName == "" || spec.PackageName == "" {
		ConfigurationHelp(errOut)
		return 2
	}
	for _, rawImport := range imports {
		item, err := parseImportSpec(rawImport)
		if err != nil {
			_, _ = fmt.Fprintf(errOut, "%v\n\n", err)
			ConfigurationHelp(errOut)
			return 2
		}
		spec.Imports = append(spec.Imports, item)
	}
	for _, rawBean := range beans {
		item, err := parseBeanSpec(rawBean)
		if err != nil {
			_, _ = fmt.Fprintf(errOut, "%v\n\n", err)
			ConfigurationHelp(errOut)
			return 2
		}
		spec.Beans = append(spec.Beans, item)
	}

	source, err := manual.GenerateConfiguration(spec)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "%v\n", err)
		return 2
	}
	if output == "" {
		_, _ = out.Write(source)
		return 0
	}
	if err := writeFile(output, source); err != nil {
		_, _ = fmt.Fprintf(errOut, "写入生成文件失败: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(errOut, "generated %s\n", output)
	return 0
}

func runRegistry(args []string, out io.Writer, errOut io.Writer) int {
	var imports stringList
	var configurations stringList
	var output string
	spec := manual.RegistrySpec{}
	flags := flag.NewFlagSet("goark codegen registry", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&spec.PackageName, "package", "", "生成文件包名")
	flags.StringVar(&spec.FunctionName, "function", "", "注册函数名，默认 RegisterConfigurations")
	flags.StringVar(&output, "output", "", "输出文件路径，留空时输出到 stdout")
	flags.Var(&imports, "import", "额外导入，格式为 path 或 alias=path，可重复")
	flags.Var(
		&configurations,
		"configuration",
		"配置类型表达式，例如 AdminConfiguration 或 cfg.AdminConfiguration，可重复",
	)

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			RegistryHelp(out)
			return 0
		}
		_, _ = fmt.Fprintf(errOut, "%v\n\n", err)
		RegistryHelp(errOut)
		return 2
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(errOut, "多余参数: %s\n\n", strings.Join(flags.Args(), " "))
		RegistryHelp(errOut)
		return 2
	}
	if spec.PackageName == "" || len(configurations) == 0 {
		RegistryHelp(errOut)
		return 2
	}
	for _, rawImport := range imports {
		item, err := parseImportSpec(rawImport)
		if err != nil {
			_, _ = fmt.Fprintf(errOut, "%v\n\n", err)
			RegistryHelp(errOut)
			return 2
		}
		spec.Imports = append(spec.Imports, item)
	}
	for _, rawConfiguration := range configurations {
		item := manual.ConfigurationRegistrationSpec{Type: strings.TrimSpace(rawConfiguration)}
		spec.Configurations = append(spec.Configurations, item)
	}

	source, err := manual.GenerateRegistry(spec)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "%v\n", err)
		return 2
	}
	if output == "" {
		_, _ = out.Write(source)
		return 0
	}
	if err := writeFile(output, source); err != nil {
		_, _ = fmt.Fprintf(errOut, "写入生成文件失败: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(errOut, "generated %s\n", output)
	return 0
}

func parseImportSpec(raw string) (manual.ImportSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return manual.ImportSpec{}, fmt.Errorf("import 不能为空")
	}
	alias, path, ok := strings.Cut(raw, "=")
	if !ok {
		return manual.ImportSpec{Path: raw}, nil
	}
	alias = strings.TrimSpace(alias)
	path = strings.TrimSpace(path)
	if alias == "" || path == "" {
		return manual.ImportSpec{}, fmt.Errorf("import %q 格式错误", raw)
	}
	return manual.ImportSpec{Alias: alias, Path: path}, nil
}

func parseBeanSpec(raw string) (manual.BeanSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return manual.BeanSpec{}, fmt.Errorf("bean 不能为空")
	}
	parts := strings.Split(raw, ";")
	name, provider, ok := strings.Cut(parts[0], "=")
	if !ok {
		return manual.BeanSpec{}, fmt.Errorf("bean %q 缺少 name=provider", raw)
	}
	bean := manual.BeanSpec{
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
				return manual.BeanSpec{}, fmt.Errorf("bean %q deps 不能为空", bean.Name)
			}
			for _, dep := range strings.Split(deps, ",") {
				bean.Dependencies = append(bean.Dependencies, strings.TrimSpace(dep))
			}
		case strings.HasPrefix(option, "scope="):
			bean.Scope = strings.TrimSpace(strings.TrimPrefix(option, "scope="))
		default:
			return manual.BeanSpec{}, fmt.Errorf("bean %q 不支持选项 %q", bean.Name, option)
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

// Help 输出 codegen 命令帮助。
func Help(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark codegen <generator> [flags]

Available generators:
  configuration     Generate a goark.Configuration source file.
  registry          Generate a Configuration registry source file.
  annotations       Scan //goark annotations and generate registration code.

`)
}

// ConfigurationHelp 输出 configuration 生成器帮助。
func ConfigurationHelp(w io.Writer) {
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
  goark codegen configuration --name user --package generated --type UserConfiguration`+
		` --order 100 --output internal/generated/user_configuration.go
  goark codegen configuration --name user --package generated`+
		` --bean "userRepository=NewUserRepository"`+
		` --bean "userService=NewUserService;deps=userRepository"

`)
}

// RegistryHelp 输出 registry 生成器帮助。
func RegistryHelp(w io.Writer) {
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
  goark codegen registry --package generated --configuration UserConfiguration`+
		` --configuration HTTPConfiguration --output internal/generated/registry.go
  goark codegen registry --package generated`+
		` --import cfg=example.com/app/internal/config`+
		` --configuration cfg.AdminConfiguration

`)
}

// AnnotationsHelp 输出 annotations 生成器帮助。
func AnnotationsHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark codegen annotations --dir <package-dir> [flags]

Flags:
  --dir path          Go package directory to scan. Defaults to current directory.
  --package string    Package name to scan when directory contains multiple packages.
  --name string       Generated configuration name when no //goark:configuration exists.
  --type string       Generated configuration type when no //goark:configuration exists.

Examples:
  goark codegen annotations --dir .
  goark codegen annotations --dir internal/app

`)
}
