package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type projectResolver struct {
	Dir        string
	Env        []string
	Runner     ProcessRunner
	Err        io.Writer
	Patterns   []string
	BuildFlags []string
}

type goarkProject struct {
	Root       string
	ModulePath string
	Packages   []goPackage
}

type goPackage struct {
	Dir        string
	ImportPath string
	Name       string
	GoFiles    []string
	CgoFiles   []string
}

type goModule struct {
	Path  string
	Dir   string
	GoMod string
}

func (r projectResolver) Resolve() (goarkProject, error) {
	r = r.withDefaults()
	module, err := r.resolveModule()
	if err != nil {
		return goarkProject{}, err
	}
	patterns := []string{"./..."}
	if len(r.Patterns) > 0 {
		patterns = append([]string(nil), r.Patterns...)
	}
	packages, err := r.listPackages(module.Dir, patterns)
	if err != nil {
		return goarkProject{}, err
	}
	return goarkProject{
		Root:       filepath.Clean(module.Dir),
		ModulePath: module.Path,
		Packages:   packages,
	}, nil
}

func (r projectResolver) withDefaults() projectResolver {
	if r.Dir == "" {
		r.Dir, _ = os.Getwd()
	}
	if r.Runner == nil {
		r.Runner = osProcessRunner{}
	}
	if r.Err == nil {
		r.Err = io.Discard
	}
	return r
}

func (r projectResolver) resolveModule() (goModule, error) {
	var output bytes.Buffer
	var diagnostic bytes.Buffer
	args := []string{"list", "-m", "-json"}
	args = append(args, r.BuildFlags...)
	err := r.Runner.Run(ProcessRequest{
		Name: "go",
		Args: args,
		Dir:  r.Dir,
		Env:  append([]string(nil), r.Env...),
		Out:  &output,
		Err:  &diagnostic,
	})
	if err != nil {
		return goModule{}, commandFailure("发现 Go 模块", err, diagnostic.String())
	}
	currentDir, err := filepath.Abs(r.Dir)
	if err != nil {
		return goModule{}, fmt.Errorf("解析当前目录失败: %w", err)
	}
	decoder := json.NewDecoder(&output)
	var selected goModule
	for {
		var module goModule
		if err := decoder.Decode(&module); err != nil {
			if err == io.EOF {
				break
			}
			return goModule{}, fmt.Errorf("解析 go list -m 输出失败: %w", err)
		}
		if strings.TrimSpace(module.Path) == "" || strings.TrimSpace(module.Dir) == "" || strings.TrimSpace(module.GoMod) == "" {
			continue
		}
		module.Dir = filepath.Clean(module.Dir)
		if !pathWithin(module.Dir, currentDir) {
			continue
		}
		if selected.Dir == "" || len(module.Dir) > len(selected.Dir) {
			selected = module
		}
	}
	if selected.Dir == "" {
		return goModule{}, fmt.Errorf("当前目录不属于可生成的本地 Go 模块")
	}
	return selected, nil
}

func (r projectResolver) listPackages(root string, patterns []string) ([]goPackage, error) {
	args := []string{"list", "-e", "-json"}
	args = append(args, r.BuildFlags...)
	args = append(args, patterns...)
	var output bytes.Buffer
	var diagnostic bytes.Buffer
	err := r.Runner.Run(ProcessRequest{
		Name: "go",
		Args: args,
		Dir:  root,
		Env:  append([]string(nil), r.Env...),
		Out:  &output,
		Err:  &diagnostic,
	})
	if err != nil {
		return nil, commandFailure("发现 Go package", err, diagnostic.String())
	}

	decoder := json.NewDecoder(&output)
	packages := make([]goPackage, 0)
	seen := make(map[string]struct{})
	for {
		var item goPackage
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("解析 go list 输出失败: %w", err)
		}
		item.Dir = filepath.Clean(item.Dir)
		if item.Dir == "." || item.Dir == "" {
			continue
		}
		if _, ok := seen[item.Dir]; ok {
			continue
		}
		seen[item.Dir] = struct{}{}
		packages = append(packages, item)
	}
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].ImportPath < packages[j].ImportPath
	})
	return packages, nil
}

func (p goarkProject) ResolveRunTarget(workingDir string) (string, error) {
	if p.isMainPackage(workingDir) {
		return ".", nil
	}

	candidates := make([]string, 0)
	commandRoot := filepath.Join(p.Root, "cmd")
	for _, item := range p.Packages {
		if item.Name != "main" || !pathWithin(commandRoot, item.Dir) {
			continue
		}
		relative, err := filepath.Rel(p.Root, item.Dir)
		if err != nil {
			continue
		}
		candidates = append(candidates, "./"+filepath.ToSlash(relative))
	}
	sort.Strings(candidates)
	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("未发现 main package，请显式指定运行目标")
	case 1:
		return candidates[0], nil
	default:
		return "", fmt.Errorf("发现多个 main package，请显式指定运行目标: %s", strings.Join(candidates, ", "))
	}
}

func (p goarkProject) isMainPackage(dir string) bool {
	dir = filepath.Clean(dir)
	for _, item := range p.Packages {
		if item.Name == "main" && samePath(item.Dir, dir) {
			return true
		}
	}
	return false
}

func commandFailure(action string, err error, diagnostic string) error {
	diagnostic = strings.TrimSpace(diagnostic)
	if diagnostic == "" {
		return fmt.Errorf("%s失败: %w", action, err)
	}
	return fmt.Errorf("%s失败: %w: %s", action, err, diagnostic)
}

func pathWithin(root string, target string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func samePath(left string, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func validateLocalProjectPattern(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("路径不能为空")
	}
	normalized := filepath.ToSlash(value)
	if normalized != "." && !strings.HasPrefix(normalized, "./") {
		return fmt.Errorf("路径 %q 必须使用 . 或 ./ 开头", value)
	}
	cleanedPath := filepath.Clean(value)
	cleaned := filepath.ToSlash(cleanedPath)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || filepath.IsAbs(value) || filepath.IsAbs(cleanedPath) {
		return fmt.Errorf("路径 %q 不能位于项目外", value)
	}
	return nil
}
