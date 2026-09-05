// Package projecttrust 管理与项目描述摘要绑定的本机信任记录。
package projecttrust

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"goark.dev/cli/internal/atomicfile"
)

const currentVersion = 1

// Store 将项目信任记录隔离在用户配置目录中。
type Store struct {
	Dir string
}

type record struct {
	Version     int    `json:"version"`
	Root        string `json:"root"`
	BuildSHA256 string `json:"buildSha256"`
}

// Default 返回当前用户的默认信任库。
func Default() (Store, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return Store{}, fmt.Errorf("解析用户配置目录失败: %w", err)
	}
	return Store{Dir: filepath.Join(directory, "goark", "trust")}, nil
}

// Trust 原子写入当前项目描述摘要的信任记录。
func (s Store) Trust(root string, digest string) error {
	if !validDigest(digest) {
		return fmt.Errorf("goark.build 摘要必须是小写十六进制 SHA-256")
	}
	canonical, err := canonicalRoot(root)
	if err != nil {
		return err
	}
	path, err := s.recordPath(canonical)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("创建项目信任目录失败: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("收紧项目信任目录权限失败: %w", err)
		}
	}
	data, err := json.Marshal(record{Version: currentVersion, Root: canonical, BuildSHA256: digest})
	if err != nil {
		return fmt.Errorf("编码项目信任记录失败: %w", err)
	}
	data = append(data, '\n')
	if err := atomicfile.Write(path, data, 0o600); err != nil {
		return fmt.Errorf("写入项目信任记录失败: %w", err)
	}
	return nil
}

// Verify 验证项目根路径和当前描述摘要均与信任记录一致。
func (s Store) Verify(root string, digest string) error {
	canonical, err := canonicalRoot(root)
	if err != nil {
		return err
	}
	path, err := s.recordPath(canonical)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("项目不受信任: %w", err)
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) || bytes.ContainsRune(data, '\r') || !utf8.Valid(data) {
		return fmt.Errorf("项目不受信任: 信任记录编码无效")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var stored record
	if err := decoder.Decode(&stored); err != nil {
		return fmt.Errorf("项目不受信任: 解析信任记录失败: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return fmt.Errorf("项目不受信任: %w", err)
	}
	if stored.Version != currentVersion || !sameRoot(stored.Root, canonical) || stored.BuildSHA256 != digest {
		return fmt.Errorf("项目不受信任: 根路径或 goark.build 摘要不匹配")
	}
	return nil
}

func (s Store) recordPath(root string) (string, error) {
	if s.Dir == "" {
		return "", fmt.Errorf("项目信任目录不能为空")
	}
	canonical, err := canonicalRoot(root)
	if err != nil {
		return "", err
	}
	identity := canonical
	if runtime.GOOS == "windows" {
		identity = strings.ToLower(identity)
	}
	digest := sha256.Sum256([]byte(filepath.ToSlash(identity)))
	return filepath.Join(s.Dir, hex.EncodeToString(digest[:])+".json"), nil
}

func canonicalRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("解析项目根目录失败: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("解析项目根目录符号链接失败: %w", err)
	}
	return filepath.Clean(canonical), nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("信任记录包含多余内容")
		}
		return fmt.Errorf("信任记录尾部无效: %w", err)
	}
	return nil
}

func sameRoot(left string, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
