package toollock

import (
	"fmt"
	"sort"
	"strings"

	"goark.dev/cli/internal/buildspec"
)

const CurrentVersion = 1

// File 是 goark.build.lock 的稳定数据模型。
type File struct {
	Version     int     `toml:"version" json:"version"`
	BuildSHA256 string  `toml:"build-sha256" json:"buildSha256"`
	Tools       []Entry `toml:"tools" json:"tools"`
}

// Entry 锁定一个工具在指定平台上的解析结果。
type Entry struct {
	Name          string             `toml:"name" json:"name"`
	Type          buildspec.ToolType `toml:"type" json:"type"`
	GOOS          string             `toml:"goos" json:"goos"`
	GOARCH        string             `toml:"goarch" json:"goarch"`
	Package       string             `toml:"package,omitempty" json:"package,omitempty"`
	Version       string             `toml:"version,omitempty" json:"version,omitempty"`
	Module        string             `toml:"module,omitempty" json:"module,omitempty"`
	ModuleVersion string             `toml:"module-version,omitempty" json:"moduleVersion,omitempty"`
	ModuleSum     string             `toml:"module-sum,omitempty" json:"moduleSum,omitempty"`
	Path          string             `toml:"path" json:"path"`
	SHA256        string             `toml:"sha256" json:"sha256"`
}

// VerifyBuild 检查锁文件是否对应当前项目描述文件。
func (f File) VerifyBuild(digest string) error {
	if f.BuildSHA256 != digest {
		return fmt.Errorf("goark.build.lock 与 goark.build 摘要不一致")
	}
	return nil
}

// Find 返回当前平台的指定工具锁定项。
func (f File) Find(name string, goos string, goarch string) (Entry, bool) {
	for _, entry := range f.Tools {
		if entry.Name == name && entry.GOOS == goos && entry.GOARCH == goarch {
			return entry, true
		}
	}
	return Entry{}, false
}

func normalize(file File) File {
	file.Tools = append([]Entry(nil), file.Tools...)
	sort.Slice(file.Tools, func(left int, right int) bool {
		if file.Tools[left].Name != file.Tools[right].Name {
			return file.Tools[left].Name < file.Tools[right].Name
		}
		if file.Tools[left].GOOS != file.Tools[right].GOOS {
			return file.Tools[left].GOOS < file.Tools[right].GOOS
		}
		return file.Tools[left].GOARCH < file.Tools[right].GOARCH
	})
	return file
}

func validate(file File) error {
	if file.Version != CurrentVersion {
		return fmt.Errorf("不支持锁文件 version = %d", file.Version)
	}
	if !validDigest(file.BuildSHA256) {
		return fmt.Errorf("build-sha256 必须是小写十六进制 SHA-256")
	}
	seen := make(map[string]struct{}, len(file.Tools))
	for _, entry := range file.Tools {
		key := entry.Name + "\x00" + entry.GOOS + "\x00" + entry.GOARCH
		if _, exists := seen[key]; exists {
			return fmt.Errorf("工具 %q 在 %s/%s 存在重复锁定项", entry.Name, entry.GOOS, entry.GOARCH)
		}
		seen[key] = struct{}{}
		if entry.Name == "" || entry.GOOS == "" || entry.GOARCH == "" || entry.Path == "" {
			return fmt.Errorf("工具锁定项缺少 name、goos、goarch 或 path")
		}
		if !validDigest(entry.SHA256) {
			return fmt.Errorf("工具 %q 的 sha256 必须是小写十六进制 SHA-256", entry.Name)
		}
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if char >= '0' && char <= '9' || char >= 'a' && char <= 'f' {
			continue
		}
		return false
	}
	return !strings.ContainsAny(value, "ABCDEF")
}
