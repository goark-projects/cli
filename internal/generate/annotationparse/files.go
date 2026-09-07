package annotationparse

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Package 保存一个已解析 Go 包的源码文件。
type Package struct {
	Name  string
	Files map[string]*ast.File
}

// ParsePackages 按当前构建上下文解析目录或显式文件列表。
func ParsePackages(
	fset *token.FileSet,
	dir string,
	files []string,
) (map[string]*Package, error) {
	if len(files) == 0 {
		matched, err := matchingFiles(dir)
		if err != nil {
			return nil, err
		}
		files = matched
	}
	packages := make(map[string]*Package)
	for _, name := range files {
		if err := parseFile(packages, fset, dir, name); err != nil {
			return nil, err
		}
	}
	return packages, nil
}

func matchingFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasSuffix(name, "_test.go") ||
			!strings.HasSuffix(name, ".go") {
			continue
		}
		matched, matchErr := build.Default.MatchFile(dir, name)
		if matchErr != nil {
			return nil, matchErr
		}
		if matched {
			files = append(files, name)
		}
	}
	return files, nil
}

func parseFile(
	packages map[string]*Package,
	fset *token.FileSet,
	dir string,
	name string,
) error {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasSuffix(name, "_test.go") ||
		!strings.HasSuffix(name, ".go") {
		return fmt.Errorf("invalid go source file %q", name)
	}
	path := filepath.Join(dir, name)
	relative, err := filepath.Rel(dir, path)
	if err != nil || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("go source file %q is outside scan directory", name)
	}
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	packageName := file.Name.Name
	parsedPackage := packages[packageName]
	if parsedPackage == nil {
		parsedPackage = &Package{
			Name:  packageName,
			Files: make(map[string]*ast.File),
		}
		packages[packageName] = parsedPackage
	}
	parsedPackage.Files[path] = file
	return nil
}

// SortedFiles 按源文件名返回稳定有序的语法树。
func SortedFiles(fset *token.FileSet, parsedPackage *Package) []*ast.File {
	files := make([]*ast.File, 0, len(parsedPackage.Files))
	for _, file := range parsedPackage.Files {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		left := fset.Position(files[i].Package).Filename
		right := fset.Position(files[j].Package).Filename
		return left < right
	})
	return files
}
