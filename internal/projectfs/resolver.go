package projectfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Existence 控制目标路径是否必须已经存在。
type Existence bool

const (
	AllowMissing Existence = false
	MustExist    Existence = true
)

// Resolver 将配置路径约束在单一项目根目录内。
type Resolver struct {
	root string
}

// New 创建项目路径解析器。
func New(root string) Resolver {
	return Resolver{root: root}
}

// Resolve 执行词法与符号链接两阶段边界检查。
func (r Resolver) Resolve(value string, existence Existence) (string, error) {
	root, err := filepath.Abs(r.root)
	if err != nil {
		return "", fmt.Errorf("解析项目根目录失败: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("解析项目根目录符号链接失败: %w", err)
	}
	if filepath.IsAbs(value) {
		return "", fmt.Errorf("路径 %q 必须位于项目根目录内且使用相对路径", value)
	}
	target := filepath.Clean(filepath.Join(root, value))
	if !within(root, target) {
		return "", fmt.Errorf("路径 %q 逃出项目根目录", value)
	}
	canonical, err := canonicalTarget(target, existence)
	if err != nil {
		return "", fmt.Errorf("解析项目路径 %q 失败: %w", value, err)
	}
	if !within(root, canonical) {
		return "", fmt.Errorf("路径 %q 经符号链接解析后逃出项目根目录", value)
	}
	return canonical, nil
}

// ResolvePattern 校验 glob 静态前缀并返回规范化模式。
func (r Resolver) ResolvePattern(pattern string) (string, error) {
	if strings.TrimSpace(pattern) == "" {
		return "", fmt.Errorf("路径模式不能为空")
	}
	prefix := patternPrefix(pattern)
	if _, err := r.Resolve(prefix, AllowMissing); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Clean(pattern)), nil
}

func canonicalTarget(target string, existence Existence) (string, error) {
	canonical, err := filepath.EvalSymlinks(target)
	if err == nil {
		return filepath.Clean(canonical), nil
	}
	if existence == MustExist || !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	ancestor := target
	suffix := make([]string, 0, 4)
	for {
		if _, statErr := os.Lstat(ancestor); statErr == nil {
			break
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", err
		}
		suffix = append(suffix, filepath.Base(ancestor))
		ancestor = parent
	}
	canonical, err = filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for index := len(suffix) - 1; index >= 0; index-- {
		canonical = filepath.Join(canonical, suffix[index])
	}
	return filepath.Clean(canonical), nil
}

func patternPrefix(pattern string) string {
	normalized := filepath.FromSlash(pattern)
	index := strings.IndexAny(normalized, "*?[")
	if index < 0 {
		return normalized
	}
	prefix := normalized[:index]
	if prefix == "" {
		return "."
	}
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix = filepath.Dir(prefix)
	}
	return prefix
}

func within(root string, target string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	if runtime.GOOS == "windows" {
		rootVolume := strings.ToUpper(filepath.VolumeName(root))
		targetVolume := strings.ToUpper(filepath.VolumeName(target))
		return rootVolume == targetVolume
	}
	return true
}
