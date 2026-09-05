// Package projectclean 删除项目显式声明的构建输出和任务缓存。
package projectclean

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/projectfs"
)

// Cleaner 在项目边界内执行确定性清理。
type Cleaner struct {
	Root     string
	Document buildspec.Document
}

// Run 返回存在且纳入清理范围的项目相对路径。
func (c Cleaner) Run(dryRun bool) ([]string, error) {
	patterns, err := c.patterns()
	if err != nil {
		return nil, err
	}
	targets, err := c.targets(patterns)
	if err != nil {
		return nil, err
	}
	if dryRun {
		return targets, nil
	}
	for _, relative := range targets {
		lexical := filepath.Join(c.Root, filepath.FromSlash(relative))
		if err := os.RemoveAll(lexical); err != nil {
			return nil, fmt.Errorf("删除 %s 失败: %w", relative, err)
		}
	}
	return targets, nil
}

func (c Cleaner) patterns() ([]string, error) {
	patterns := []string{".goark/cache"}
	for _, command := range c.Document.Commands {
		if command.Output != "" {
			patterns = append(patterns, command.Output)
		}
	}
	for _, task := range c.Document.Tasks {
		patterns = append(patterns, task.Outputs...)
	}
	seen := make(map[string]struct{}, len(patterns))
	result := make([]string, 0, len(patterns))
	resolver := projectfs.New(c.Root)
	for _, pattern := range patterns {
		if strings.Contains(pattern, "${") {
			return nil, fmt.Errorf("clean 无法静态解析包含变量的输出模式 %q", pattern)
		}
		normalized, err := resolver.ResolvePattern(pattern)
		if err != nil {
			return nil, err
		}
		if normalized == "." {
			return nil, fmt.Errorf("禁止清理项目根目录")
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result, nil
}

func (c Cleaner) targets(patterns []string) ([]string, error) {
	resolver := projectfs.New(c.Root)
	seen := make(map[string]struct{})
	for _, pattern := range patterns {
		matches, err := doublestar.Glob(os.DirFS(c.Root), pattern)
		if err != nil {
			return nil, fmt.Errorf("清理模式 %q 无效: %w", pattern, err)
		}
		// 精确路径需要覆盖空目录等不包含子项的输出。
		if !strings.ContainsAny(pattern, "*?[") {
			if _, err := os.Lstat(filepath.Join(c.Root, filepath.FromSlash(pattern))); err == nil {
				matches = append(matches, pattern)
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
		for _, match := range matches {
			if _, err := resolver.Resolve(match, projectfs.MustExist); err != nil {
				return nil, err
			}
			normalized := filepath.ToSlash(filepath.Clean(match))
			if normalized == "." {
				return nil, fmt.Errorf("禁止清理项目根目录")
			}
			seen[normalized] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for target := range seen {
		result = append(result, target)
	}
	sort.Slice(result, func(i, j int) bool {
		if depthI, depthJ := strings.Count(result[i], "/"), strings.Count(result[j], "/"); depthI != depthJ {
			return depthI > depthJ
		}
		return result[i] < result[j]
	})
	return result, nil
}
