package taskcache

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/bmatcuk/doublestar/v4"
	"goark.dev/cli/internal/projectfs"
)

type fileDigest struct {
	Path   string `json:"path"`
	Mode   uint32 `json:"mode"`
	SHA256 string `json:"sha256"`
}

func collect(root string, patterns []string, requireEach bool) ([]fileDigest, error) {
	resolver := projectfs.New(root)
	files := make(map[string]fileDigest)
	for _, pattern := range patterns {
		normalized, err := resolver.ResolvePattern(pattern)
		if err != nil {
			return nil, err
		}
		matches, err := doublestar.Glob(os.DirFS(root), normalized)
		if err != nil {
			return nil, fmt.Errorf("路径模式 %q 无效: %w", pattern, err)
		}
		if requireEach && len(matches) == 0 {
			return nil, fmt.Errorf("outputs 模式 %q 没有匹配项", pattern)
		}
		for _, match := range matches {
			resolved, err := resolver.Resolve(match, projectfs.MustExist)
			if err != nil {
				return nil, err
			}
			if err := collectPath(root, resolved, files); err != nil {
				return nil, err
			}
		}
	}
	result := make([]fileDigest, 0, len(files))
	for _, item := range files {
		result = append(result, item)
	}
	sort.Slice(result, func(left int, right int) bool { return result[left].Path < result[right].Path })
	return result, nil
}

func collectPath(root string, target string, files map[string]fileDigest) error {
	return filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			relativeLink, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			resolved, err := projectfs.New(root).Resolve(relativeLink, projectfs.MustExist)
			if err != nil {
				return err
			}
			path = resolved
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		item := fileDigest{Path: filepath.ToSlash(relative), Mode: uint32(info.Mode().Perm())}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			item.SHA256 = hashBytes(data)
		}
		files[item.Path] = item
		return nil
	})
}

// OutputDigest 返回任务当前输出集合的稳定摘要。
func OutputDigest(context Context) (string, error) {
	outputs, err := collect(context.Root, context.Task.Outputs, true)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(outputs)
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}
