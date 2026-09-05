// Package toolservice 编排工具解析、安装、锁定验证和项目信任。
package toolservice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"goark.dev/cli/internal/buildspec"
	"goark.dev/cli/internal/projectlock"
	"goark.dev/cli/internal/projecttrust"
	"goark.dev/cli/internal/tooling"
	"goark.dev/cli/internal/toollock"
)

// SyncOptions 控制锁文件同步的副作用边界。
type SyncOptions struct {
	Locked  bool
	Offline bool
}

// Service 管理一个项目的外部工具状态。
type Service struct {
	Root        string
	Document    buildspec.Document
	Environment map[string]string
	Manager     tooling.Manager
	Trust       projecttrust.Store
	GOOS        string
	GOARCH      string
}

// Status 描述一个工具在当前平台上的只读状态。
type Status struct {
	Name   string             `json:"name"`
	Type   buildspec.ToolType `json:"type"`
	Status string             `json:"status"`
	Path   string             `json:"path,omitempty"`
	Detail string             `json:"detail,omitempty"`
}

// Sync 解析全部工具并更新当前平台锁项；锁定模式只验证。
func (s Service) Sync(ctx context.Context, options SyncOptions) (result toollock.File, resultErr error) {
	if options.Locked {
		return s.Verify(ctx)
	}
	lock, err := projectlock.Acquire(ctx, s.Root)
	if err != nil {
		return toollock.File{}, err
	}
	defer func() {
		if err := lock.Release(); err != nil && resultErr == nil {
			resultErr = err
		}
	}()
	return s.sync(ctx, options)
}

func (s Service) sync(ctx context.Context, options SyncOptions) (toollock.File, error) {
	digest, err := s.buildDigest()
	if err != nil {
		return toollock.File{}, err
	}
	locked, err := s.readExistingLock()
	if err != nil {
		return toollock.File{}, err
	}
	resolved, err := s.resolve(ctx, sortedToolNames(s.Document.Tools), !options.Offline, false, options.Offline)
	if err != nil {
		return toollock.File{}, err
	}
	updated := toollock.File{
		Version: toollock.CurrentVersion, BuildSHA256: digest,
		Tools: mergePlatformEntries(locked.Tools, resolvedEntries(resolved), s.GOOS, s.GOARCH),
	}
	if err := toollock.Write(s.Root, updated); err != nil {
		return toollock.File{}, err
	}
	if err := s.Trust.Trust(s.Root, digest); err != nil {
		return toollock.File{}, fmt.Errorf("建立项目信任失败: %w", err)
	}
	return toollock.Read(s.Root)
}

// Verify 检查描述摘要、当前平台锁项和工具文件内容。
func (s Service) Verify(ctx context.Context) (toollock.File, error) {
	digest, err := s.buildDigest()
	if err != nil {
		return toollock.File{}, err
	}
	locked, err := toollock.Read(s.Root)
	if err != nil {
		return toollock.File{}, err
	}
	if err := locked.VerifyBuild(digest); err != nil {
		return toollock.File{}, err
	}
	names := sortedToolNames(s.Document.Tools)
	resolved, err := s.resolve(ctx, names, false, false, true)
	if err != nil {
		return toollock.File{}, err
	}
	for _, name := range names {
		entry, ok := locked.Find(name, s.GOOS, s.GOARCH)
		if !ok {
			return toollock.File{}, fmt.Errorf("工具 %q 缺少 %s/%s 锁定项", name, s.GOOS, s.GOARCH)
		}
		if !matchesDeclaration(entry, s.Document.Tools[name]) {
			return toollock.File{}, fmt.Errorf("工具 %q 的声明与锁定项不一致", name)
		}
		if err := tooling.Verify(resolved[name], entry); err != nil {
			return toollock.File{}, err
		}
	}
	for _, entry := range locked.Tools {
		if entry.GOOS != s.GOOS || entry.GOARCH != s.GOARCH {
			continue
		}
		if _, ok := s.Document.Tools[entry.Name]; !ok {
			return toollock.File{}, fmt.Errorf("锁文件包含未声明工具 %q 的 %s/%s 锁定项", entry.Name, s.GOOS, s.GOARCH)
		}
	}
	return locked, nil
}

// Install 显式安装或解析指定工具，并合并当前平台锁项。
func (s Service) Install(ctx context.Context, name string, offline bool) (result tooling.Resolved, resultErr error) {
	if _, ok := s.Document.Tools[name]; !ok {
		return tooling.Resolved{}, fmt.Errorf("工具 %q 不存在", name)
	}
	lock, err := projectlock.Acquire(ctx, s.Root)
	if err != nil {
		return tooling.Resolved{}, err
	}
	defer func() {
		if err := lock.Release(); err != nil && resultErr == nil {
			resultErr = err
		}
	}()
	digest, err := s.buildDigest()
	if err != nil {
		return tooling.Resolved{}, err
	}
	locked, err := s.readExistingLock()
	if err != nil {
		return tooling.Resolved{}, err
	}
	resolved, err := s.resolve(ctx, []string{name}, false, !offline, offline)
	if err != nil {
		return tooling.Resolved{}, err
	}
	updated := toollock.File{
		Version: toollock.CurrentVersion, BuildSHA256: digest,
		Tools: mergeOneEntry(locked.Tools, resolved[name].Entry),
	}
	if err := toollock.Write(s.Root, updated); err != nil {
		return tooling.Resolved{}, err
	}
	if err := s.Trust.Trust(s.Root, digest); err != nil {
		return tooling.Resolved{}, fmt.Errorf("建立项目信任失败: %w", err)
	}
	return resolved[name], nil
}

// Statuses 返回全部声明工具的当前状态，不安装、不写文件。
func (s Service) Statuses(ctx context.Context) []Status {
	digest, digestErr := s.buildDigest()
	locked, lockErr := toollock.Read(s.Root)
	if lockErr == nil {
		lockErr = locked.VerifyBuild(digest)
	}
	manager := s.Manager
	manager.GOOS = s.GOOS
	manager.GOARCH = s.GOARCH
	result := make([]Status, 0, len(s.Document.Tools))
	for _, name := range sortedToolNames(s.Document.Tools) {
		tool := s.Document.Tools[name]
		status := Status{Name: name, Type: tool.Type, Status: "missing"}
		if digestErr != nil {
			status.Status = "error"
			status.Detail = digestErr.Error()
			result = append(result, status)
			continue
		}
		item, err := manager.Resolve(ctx, name, tool, tooling.ResolveOptions{Offline: true})
		if err != nil {
			status.Detail = err.Error()
			result = append(result, status)
			continue
		}
		status.Path = item.Path
		entry, ok := locked.Find(name, s.GOOS, s.GOARCH)
		switch {
		case lockErr != nil:
			status.Status = "unlocked"
			status.Detail = lockErr.Error()
		case !ok:
			status.Status = "unlocked"
			status.Detail = "缺少当前平台锁定项"
		case !matchesDeclaration(entry, tool):
			status.Status = "drift"
			status.Detail = "声明与锁定项不一致"
		case tooling.Verify(item, entry) != nil:
			status.Status = "drift"
			status.Detail = "工具文件与锁定项不一致"
		default:
			status.Status = "ready"
		}
		result = append(result, status)
	}
	return result
}

func (s Service) resolve(ctx context.Context, names []string, allowAutoInstall bool, forceInstall bool, offline bool) (map[string]tooling.Resolved, error) {
	manager := s.Manager
	manager.GOOS = s.GOOS
	manager.GOARCH = s.GOARCH
	result := make(map[string]tooling.Resolved, len(names))
	for _, name := range names {
		tool := s.Document.Tools[name]
		item, err := manager.Resolve(ctx, name, tool, tooling.ResolveOptions{
			AllowInstall: forceInstall || allowAutoInstall && tool.Install == "auto", ForceInstall: forceInstall, Offline: offline,
		})
		if err != nil {
			return nil, err
		}
		result[name] = item
	}
	return result, nil
}

func (s Service) buildDigest() (string, error) {
	return toollock.DigestFile(filepath.Join(s.Root, buildspec.FileName))
}

func (s Service) readExistingLock() (toollock.File, error) {
	path := filepath.Join(s.Root, buildspec.LockFileName)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return toollock.File{Version: toollock.CurrentVersion}, nil
		}
		return toollock.File{}, fmt.Errorf("检查锁文件失败: %w", err)
	}
	return toollock.Read(s.Root)
}

func sortedToolNames(tools map[string]buildspec.Tool) []string {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func resolvedEntries(resolved map[string]tooling.Resolved) []toollock.Entry {
	names := make([]string, 0, len(resolved))
	for name := range resolved {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]toollock.Entry, 0, len(names))
	for _, name := range names {
		entries = append(entries, resolved[name].Entry)
	}
	return entries
}

func mergePlatformEntries(existing []toollock.Entry, current []toollock.Entry, goos string, goarch string) []toollock.Entry {
	result := make([]toollock.Entry, 0, len(existing)+len(current))
	for _, entry := range existing {
		if entry.GOOS != goos || entry.GOARCH != goarch {
			result = append(result, entry)
		}
	}
	return append(result, current...)
}

func mergeOneEntry(existing []toollock.Entry, current toollock.Entry) []toollock.Entry {
	result := make([]toollock.Entry, 0, len(existing)+1)
	for _, entry := range existing {
		if entry.Name != current.Name || entry.GOOS != current.GOOS || entry.GOARCH != current.GOARCH {
			result = append(result, entry)
		}
	}
	return append(result, current)
}

func matchesDeclaration(entry toollock.Entry, tool buildspec.Tool) bool {
	return entry.Type == tool.Type && entry.Package == tool.Package && entry.Version == tool.Version
}
