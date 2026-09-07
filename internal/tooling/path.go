package tooling

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"goark.dev/cli/internal/envutil"
)

func lookPath(command string, environment map[string]string) (string, error) {
	if command == "" || strings.ContainsAny(command, `/\\`) {
		return "", fmt.Errorf("系统工具 command 必须是不含路径分隔符的命令名")
	}
	pathValue, _ := envutil.Lookup(environment, "PATH")
	if pathValue == "" {
		pathValue = os.Getenv("PATH")
	}
	extensions := []string{""}
	if runtime.GOOS == "windows" && filepath.Ext(command) == "" {
		pathExt, _ := envutil.Lookup(environment, "PATHEXT")
		if pathExt == "" {
			pathExt = ".COM;.EXE;.BAT;.CMD"
		}
		extensions = filepath.SplitList(pathExt)
	}
	for _, directory := range filepath.SplitList(pathValue) {
		if directory == "" {
			continue
		}
		for _, extension := range extensions {
			candidate := filepath.Join(directory, command+extension)
			if executableFile(candidate) {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("在 PATH 中找不到 %q", command)
}

func canonicalExecutable(file string) (string, error) {
	return canonicalExecutablePath(file, filepath.EvalSymlinks)
}

func canonicalExecutablePath(
	file string,
	evaluate func(string) (string, error),
) (string, error) {
	absolute, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}
	if !executableFile(absolute) {
		return "", fmt.Errorf("%s 不是可执行普通文件", absolute)
	}
	canonical, err := evaluate(absolute)
	if err != nil {
		// Windows 托管环境可能通过目录链接提供工具，但无法展开完整链接链。
		return filepath.Clean(absolute), nil
	}
	if !executableFile(canonical) {
		return "", fmt.Errorf("%s 不是可执行普通文件", canonical)
	}
	return filepath.Clean(canonical), nil
}

func executableFile(file string) bool {
	info, err := os.Stat(file)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0
}
