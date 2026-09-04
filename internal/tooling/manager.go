package tooling

import (
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/projectfs"
	"goark.dev/cli/internal/toollock"
)

// BuildMetadata 描述 Go 可执行文件的主模块身份。
type BuildMetadata struct {
	Module  string
	Version string
	Sum     string
}

// ResolveOptions 控制工具解析时允许的副作用。
type ResolveOptions struct {
	AllowInstall bool
	Offline      bool
}

// Resolved 是工具解析和摘要计算后的不可变结果。
type Resolved struct {
	Name  string
	Path  string
	Entry toollock.Entry
}

// Manager 统一解析 Go、系统和项目本地工具。
type Manager struct {
	Root        string
	CacheDir    string
	Environment map[string]string
	GOOS        string
	GOARCH      string
	LookPath    func(string, map[string]string) (string, error)
	InstallGo   func(context.Context, string, string, string, map[string]string) error
	ReadBuild   func(string) (BuildMetadata, error)
}

// NewManager 创建带生产默认实现的工具管理器。
func NewManager(root string, cacheDir string, environment map[string]string) Manager {
	return Manager{
		Root:        root,
		CacheDir:    cacheDir,
		Environment: cloneEnvironment(environment),
		GOOS:        runtime.GOOS,
		GOARCH:      runtime.GOARCH,
		LookPath:    lookPath,
		InstallGo:   installGo,
		ReadBuild:   readBuildMetadata,
	}
}

// Resolve 校验并解析一个工具。
func (m Manager) Resolve(ctx context.Context, name string, tool buildspec.Tool, options ResolveOptions) (Resolved, error) {
	switch tool.Type {
	case buildspec.ToolTypeSystem:
		return m.resolveSystem(name, tool)
	case buildspec.ToolTypeLocal:
		return m.resolveLocal(name, tool)
	case buildspec.ToolTypeGo:
		return m.resolveGo(ctx, name, tool, options)
	default:
		return Resolved{}, fmt.Errorf("工具 %q 的类型 %q 不受支持", name, tool.Type)
	}
}

func (m Manager) resolveSystem(name string, tool buildspec.Tool) (Resolved, error) {
	resolvedPath, err := m.LookPath(tool.Command, m.Environment)
	if err != nil {
		return Resolved{}, fmt.Errorf("系统工具 %q 不可用: %w", name, err)
	}
	resolvedPath, err = canonicalExecutable(resolvedPath)
	if err != nil {
		return Resolved{}, fmt.Errorf("系统工具 %q 无效: %w", name, err)
	}
	return m.resolved(name, tool, resolvedPath, resolvedPath, BuildMetadata{})
}

func (m Manager) resolveLocal(name string, tool buildspec.Tool) (Resolved, error) {
	resolvedPath, err := projectfs.New(m.Root).Resolve(tool.Path, projectfs.MustExist)
	if err != nil {
		return Resolved{}, fmt.Errorf("本地工具 %q 无效: %w", name, err)
	}
	resolvedPath, err = canonicalExecutable(resolvedPath)
	if err != nil {
		return Resolved{}, fmt.Errorf("本地工具 %q 无效: %w", name, err)
	}
	return m.resolved(name, tool, resolvedPath, filepath.ToSlash(filepath.Clean(tool.Path)), BuildMetadata{})
}

func (m Manager) resolveGo(ctx context.Context, name string, tool buildspec.Tool, options ResolveOptions) (Resolved, error) {
	key := toolCacheKey(tool.Package, tool.Version, m.GOOS, m.GOARCH)
	logicalPath := path.Join("go", key, "bin", executableName(path.Base(tool.Package)))
	resolvedPath := filepath.Join(m.CacheDir, filepath.FromSlash(logicalPath))
	if _, err := os.Stat(resolvedPath); err != nil {
		if !os.IsNotExist(err) {
			return Resolved{}, fmt.Errorf("检查 Go 工具 %q 失败: %w", name, err)
		}
		if options.Offline {
			return Resolved{}, fmt.Errorf("离线模式下 Go 工具 %q 不存在", name)
		}
		if !options.AllowInstall {
			return Resolved{}, fmt.Errorf("Go 工具 %q 尚未安装，请执行 goark sync 或 goark tool install %s", name, name)
		}
		if err := m.installGoCached(ctx, tool, key); err != nil {
			return Resolved{}, fmt.Errorf("安装 Go 工具 %q 失败: %w", name, err)
		}
	}
	resolvedPath, err := canonicalExecutable(resolvedPath)
	if err != nil {
		return Resolved{}, fmt.Errorf("Go 工具 %q 无效: %w", name, err)
	}
	metadata, err := m.ReadBuild(resolvedPath)
	if err != nil {
		return Resolved{}, fmt.Errorf("读取 Go 工具 %q 构建信息失败: %w", name, err)
	}
	return m.resolved(name, tool, resolvedPath, logicalPath, metadata)
}

func (m Manager) resolved(name string, tool buildspec.Tool, resolvedPath string, lockPath string, metadata BuildMetadata) (Resolved, error) {
	digest, err := digestFile(resolvedPath)
	if err != nil {
		return Resolved{}, err
	}
	entry := toollock.Entry{
		Name:          name,
		Type:          tool.Type,
		GOOS:          m.GOOS,
		GOARCH:        m.GOARCH,
		Package:       tool.Package,
		Version:       tool.Version,
		Module:        metadata.Module,
		ModuleVersion: metadata.Version,
		ModuleSum:     metadata.Sum,
		Path:          filepath.ToSlash(lockPath),
		SHA256:        digest,
	}
	return Resolved{Name: name, Path: resolvedPath, Entry: entry}, nil
}

// Verify 检查当前解析结果是否与锁定项完全一致。
func Verify(resolved Resolved, locked toollock.Entry) error {
	actual := resolved.Entry
	if actual.Name != locked.Name || actual.Type != locked.Type || actual.GOOS != locked.GOOS || actual.GOARCH != locked.GOARCH ||
		actual.Package != locked.Package || actual.Version != locked.Version || actual.Module != locked.Module ||
		actual.ModuleVersion != locked.ModuleVersion || actual.ModuleSum != locked.ModuleSum || actual.Path != locked.Path {
		return fmt.Errorf("工具 %q 的锁定元数据不一致", resolved.Name)
	}
	if actual.SHA256 != locked.SHA256 {
		return fmt.Errorf("工具 %q 的可执行文件摘要不一致", resolved.Name)
	}
	return nil
}

func readBuildMetadata(file string) (BuildMetadata, error) {
	info, err := buildinfo.ReadFile(file)
	if err != nil {
		return BuildMetadata{}, err
	}
	return BuildMetadata{Module: info.Main.Path, Version: info.Main.Version, Sum: info.Main.Sum}, nil
}

func digestFile(file string) (string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func cloneEnvironment(environment map[string]string) map[string]string {
	result := make(map[string]string, len(environment))
	for name, value := range environment {
		result[name] = value
	}
	return result
}

func environmentList(environment map[string]string) []string {
	names := make([]string, 0, len(environment))
	for name := range environment {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, name+"="+environment[name])
	}
	return result
}

func executableName(name string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		return name + ".exe"
	}
	return name
}
