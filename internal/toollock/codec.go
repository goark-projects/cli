package toollock

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	toml "github.com/pelletier/go-toml/v2"
	"goark.dev/cli/internal/atomicfile"
	"goark.dev/cli/internal/buildspec"
)

// Read 严格读取项目工具锁文件。
func Read(root string) (File, error) {
	path := filepath.Join(root, buildspec.LockFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("读取 %s 失败: %w", buildspec.LockFileName, err)
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) || !utf8.Valid(data) || bytes.ContainsRune(data, '\r') {
		return File{}, fmt.Errorf("%s 必须使用 UTF-8 无 BOM、LF", buildspec.LockFileName)
	}
	var file File
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		if bytes.Contains([]byte(err.Error()), []byte("strict mode")) {
			return File{}, fmt.Errorf("%s 包含未知字段: %w", buildspec.LockFileName, err)
		}
		return File{}, fmt.Errorf("解析 %s 失败: %w", buildspec.LockFileName, err)
	}
	file = normalize(file)
	if err := validate(file); err != nil {
		return File{}, fmt.Errorf("校验 %s 失败: %w", buildspec.LockFileName, err)
	}
	return file, nil
}

// Write 以原子替换方式写入稳定排序的工具锁文件。
func Write(root string, file File) error {
	file = normalize(file)
	if err := validate(file); err != nil {
		return fmt.Errorf("校验 %s 失败: %w", buildspec.LockFileName, err)
	}
	var data bytes.Buffer
	encoder := toml.NewEncoder(&data)
	if err := encoder.Encode(file); err != nil {
		return fmt.Errorf("编码 %s 失败: %w", buildspec.LockFileName, err)
	}
	if bytes.ContainsRune(data.Bytes(), '\r') {
		return fmt.Errorf("编码器生成了非 LF 换行")
	}
	return atomicfile.Write(filepath.Join(root, buildspec.LockFileName), data.Bytes(), 0o644)
}

// DigestFile 返回文件内容的小写 SHA-256。
func DigestFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
