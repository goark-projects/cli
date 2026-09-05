package cli

import (
	"errors"
	"fmt"
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

// resolveModuleStatic 仅通过文件系统发现当前模块，供禁止启动子进程的只读计划使用。
func (r projectResolver) resolveModuleStatic() (goModule, error) {
	directory, err := filepath.Abs(r.Dir)
	if err != nil {
		return goModule{}, fmt.Errorf("解析当前目录失败: %w", err)
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		return goModule{}, fmt.Errorf("解析当前目录符号链接失败: %w", err)
	}
	for {
		path := filepath.Join(directory, "go.mod")
		data, readErr := os.ReadFile(path)
		switch {
		case readErr == nil:
			modulePath := modfile.ModulePath(data)
			if modulePath == "" {
				return goModule{}, fmt.Errorf("go.mod 缺少有效 module 指令: %s", path)
			}
			return goModule{Path: modulePath, Dir: directory, GoMod: path}, nil
		case !errors.Is(readErr, os.ErrNotExist):
			return goModule{}, fmt.Errorf("读取 go.mod 失败: %w", readErr)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return goModule{}, fmt.Errorf("当前目录不属于可生成的本地 Go 模块")
		}
		directory = parent
	}
}

func (r projectResolver) listPackagesStatic(root string, modulePath string, patterns []string) ([]goPackage, error) {
	buildContext := build.Default
	buildContext.BuildTags = buildTags(r.BuildFlags)
	if value := environmentValue(r.Env, "GOOS"); value != "" {
		buildContext.GOOS = value
	}
	if value := environmentValue(r.Env, "GOARCH"); value != "" {
		buildContext.GOARCH = value
	}
	packages := make([]goPackage, 0)
	err := filepath.WalkDir(root, func(directory string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if directory != root && shouldSkipPackageDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		if directory != root {
			if _, statErr := os.Stat(filepath.Join(directory, "go.mod")); statErr == nil {
				return filepath.SkipDir
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return statErr
			}
		}
		relative, err := filepath.Rel(root, directory)
		if err != nil {
			return err
		}
		packagePattern := "."
		if relative != "." {
			packagePattern = "./" + filepath.ToSlash(relative)
		}
		if !matchesAnyPackagePattern(packagePattern, patterns) {
			return nil
		}
		item, err := buildContext.ImportDir(directory, 0)
		if _, noGo := err.(*build.NoGoError); noGo {
			return nil
		}
		if err != nil {
			return fmt.Errorf("解析 package %s 失败: %w", packagePattern, err)
		}
		importPath := modulePath
		if relative != "." {
			importPath += "/" + filepath.ToSlash(relative)
		}
		packages = append(packages, goPackage{
			Dir: directory, ImportPath: importPath, Name: item.Name,
			GoFiles: append([]string(nil), item.GoFiles...), CgoFiles: append([]string(nil), item.CgoFiles...),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("静态发现 Go package 失败: %w", err)
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].ImportPath < packages[j].ImportPath })
	return packages, nil
}

func shouldSkipPackageDirectory(name string) bool {
	return name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func matchesAnyPackagePattern(packagePath string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(filepath.Clean(pattern))
		switch {
		case pattern == packagePath:
			return true
		case pattern == "..." || pattern == "./...":
			return true
		case strings.HasSuffix(pattern, "/..."):
			prefix := strings.TrimSuffix(pattern, "/...")
			if packagePath == prefix || strings.HasPrefix(packagePath, prefix+"/") {
				return true
			}
		}
	}
	return false
}

func environmentValue(environment []string, name string) string {
	for index := len(environment) - 1; index >= 0; index-- {
		key, value, ok := strings.Cut(environment[index], "=")
		if ok && strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}
