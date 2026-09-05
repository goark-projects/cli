package tooling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"goark.dev/cli/internal/buildspec"
)

func (m Manager) installGoCached(ctx context.Context, tool buildspec.Tool, key string, force bool) error {
	goCache := filepath.Join(m.CacheDir, "go")
	if err := os.MkdirAll(goCache, 0o755); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(goCache, key+".lock"))
	locked, err := lock.TryLockContext(ctx, 25*time.Millisecond)
	if err != nil {
		return err
	}
	if !locked {
		return ctx.Err()
	}
	defer func() { _ = lock.Unlock() }()

	target := filepath.Join(goCache, key)
	executable := filepath.Join(target, "bin", executableName(path.Base(tool.Package)))
	if _, err := os.Stat(executable); err == nil && !force {
		return nil
	}
	temp, err := os.MkdirTemp(goCache, "."+key+"-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(temp) }()
	destination := filepath.Join(temp, "bin")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	if err := m.InstallGo(ctx, tool.Package, tool.Version, destination, m.Environment); err != nil {
		return err
	}
	if _, err := canonicalExecutable(filepath.Join(destination, executableName(path.Base(tool.Package)))); err != nil {
		return fmt.Errorf("安装结果缺少预期可执行文件: %w", err)
	}
	if err := publishGoToolCache(goCache, key, temp, target); err != nil {
		return err
	}
	return nil
}

func publishGoToolCache(cacheRoot string, key string, source string, target string) error {
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		return os.Rename(source, target)
	} else if err != nil {
		return err
	}

	backup, err := os.MkdirTemp(cacheRoot, "."+key+".stale-*")
	if err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	if err := os.Rename(target, backup); err != nil {
		return err
	}
	if err := os.Rename(source, target); err != nil {
		if rollbackErr := os.Rename(backup, target); rollbackErr != nil {
			return fmt.Errorf("发布工具缓存失败: %w；回滚旧缓存失败: %v", err, rollbackErr)
		}
		return err
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("清理旧工具缓存失败: %w", err)
	}
	return nil
}

func installGo(ctx context.Context, packagePath string, version string, destination string, environment map[string]string) error {
	command := exec.CommandContext(ctx, "go", "install", packagePath+"@"+version)
	command.Env = append(environmentList(environment), "GOBIN="+destination, "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go install 失败: %w: %s", err, output)
	}
	return nil
}

func toolCacheKey(packagePath string, version string, goos string, goarch string) string {
	digest := sha256Bytes(packagePath + "\x00" + version + "\x00" + goos + "\x00" + goarch)
	return digest
}

func sha256Bytes(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
