package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectType 表示项目骨架类型。
type ProjectType string

const (
	// ProjectTypeApp 表示不包含 HTTP 服务的 Goark Boot 应用。
	ProjectTypeApp ProjectType = "app"
	// ProjectTypeWeb 表示包含 Web 服务的 Goark Boot 应用。
	ProjectTypeWeb ProjectType = "web"
)

// AppSpec 描述 Goark 应用骨架生成参数。
type AppSpec struct {
	Dir        string
	ModulePath string
	Name       string
	Type       ProjectType
	Force      bool
}

// File 描述已写入的骨架文件。
type File struct {
	Path string
}

type appSpec struct {
	dir         string
	modulePath  string
	name        string
	projectType ProjectType
	force       bool
}

type fileSpec struct {
	path    string
	content string
}

// CreateApp 创建 Goark 应用骨架。
func CreateApp(spec AppSpec) ([]File, error) {
	normalized, err := normalizeAppSpec(spec)
	if err != nil {
		return nil, err
	}
	files := appFiles(normalized)
	if err := validateScaffoldTargets(normalized.dir, files, normalized.force); err != nil {
		return nil, err
	}
	written := make([]File, 0, len(files))
	for _, file := range files {
		if err := writeScaffoldFile(normalized.dir, file, normalized.force); err != nil {
			return nil, err
		}
		written = append(written, File{Path: filepath.Clean(file.path)})
	}
	return written, nil
}

func validateScaffoldTargets(root string, files []fileSpec, force bool) error {
	if force {
		return nil
	}
	for _, file := range files {
		target := filepath.Join(root, filepath.Clean(file.path))
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("file %s already exists", target)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func normalizeAppSpec(spec AppSpec) (appSpec, error) {
	dir := strings.TrimSpace(spec.Dir)
	if dir == "" {
		dir = "."
	}
	modulePath := strings.TrimSpace(spec.ModulePath)
	if err := validateModulePath(modulePath); err != nil {
		return appSpec{}, err
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = defaultAppName(modulePath)
	}
	projectType := spec.Type
	if projectType == "" {
		projectType = ProjectTypeApp
	}
	if projectType != ProjectTypeApp && projectType != ProjectTypeWeb {
		return appSpec{}, fmt.Errorf("unsupported project type %q", projectType)
	}
	return appSpec{
		dir:         filepath.Clean(dir),
		modulePath:  modulePath,
		name:        name,
		projectType: projectType,
		force:       spec.Force,
	}, nil
}

func validateModulePath(modulePath string) error {
	if modulePath == "" {
		return fmt.Errorf("module path is required")
	}
	if strings.ContainsAny(modulePath, " \t\r\n\"'") || strings.Contains(modulePath, "\\") {
		return fmt.Errorf("module path %q is invalid", modulePath)
	}
	if strings.HasPrefix(modulePath, ".") || strings.HasPrefix(modulePath, "/") {
		return fmt.Errorf("module path %q must be an import path", modulePath)
	}
	return nil
}

func defaultAppName(modulePath string) string {
	modulePath = strings.Trim(modulePath, "/")
	if modulePath == "" {
		return "goark-app"
	}
	parts := strings.Split(modulePath, "/")
	return sanitizeAppName(parts[len(parts)-1])
}

func sanitizeAppName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var builder strings.Builder
	lastDash := false
	for _, r := range name {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if valid {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(builder.String(), "-")
	if out == "" {
		return "goark-app"
	}
	return out
}

func writeScaffoldFile(root string, file fileSpec, force bool) error {
	target := filepath.Join(root, filepath.Clean(file.path))
	if !force {
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("file %s already exists", target)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, []byte(file.content), 0o644)
}
