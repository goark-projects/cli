package atomicfile

import (
	"os"
	"path/filepath"
)

// Write 通过同目录临时文件和原子替换写入完整内容。
func Write(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	replaced := false
	defer func() {
		if !replaced {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		return err
	}
	if err := replace(tempPath, path); err != nil {
		return err
	}
	replaced = true
	return nil
}
