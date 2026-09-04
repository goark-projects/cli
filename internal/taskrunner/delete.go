package taskrunner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
	"goark.dev/cli/internal/projectfs"
)

func deleteOutputs(root string, patterns []string) error {
	resolver := projectfs.New(root)
	for _, pattern := range patterns {
		normalized, err := resolver.ResolvePattern(pattern)
		if err != nil {
			return err
		}
		matches, err := doublestar.Glob(os.DirFS(root), normalized)
		if err != nil {
			return fmt.Errorf("删除模式 %q 无效: %w", pattern, err)
		}
		for _, match := range matches {
			target, err := resolver.Resolve(match, projectfs.MustExist)
			if err != nil {
				return err
			}
			if samePath(root, target) {
				return fmt.Errorf("禁止删除项目根目录")
			}
			if err := os.RemoveAll(target); err != nil {
				return fmt.Errorf("删除 %s 失败: %w", target, err)
			}
		}
	}
	return nil
}

func samePath(left string, right string) bool {
	left, _ = filepath.Abs(left)
	right, _ = filepath.Abs(right)
	return filepath.Clean(left) == filepath.Clean(right)
}
