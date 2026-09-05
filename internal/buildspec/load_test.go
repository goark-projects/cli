package buildspec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFile_whenDocumentIsValid_shouldApplyDefaults(t *testing.T) {
	path := writeBuildFile(t, `version = 1

[project]
name = "admin-minimal"
main = "./cmd/admin"

[commands.build]
after = ["checksum"]
go-args = ["-trimpath"]
output = "./build/admin-minimal"

[tools.sha256]
type = "system"
command = "sha256sum"
install = "manual"

[tasks.checksum]
type = "exec"
tool = "sha256"
args = ["${command.output}"]
cache = false

[profiles.production]
go-args = ["-tags=production"]
`)

	document, err := LoadFile(path)
	if err != nil {
		t.Fatalf("加载 goark.build 失败: %v", err)
	}
	if document.Version != CurrentVersion {
		t.Fatalf("version = %d", document.Version)
	}
	if document.Execution.MaxParallel <= 0 {
		t.Fatalf("max-parallel = %d", document.Execution.MaxParallel)
	}
	if document.Execution.DefaultTimeout.Duration <= 0 {
		t.Fatalf("default-timeout = %s", document.Execution.DefaultTimeout)
	}
	if got := document.Generate.Patterns; len(got) != 1 || got[0] != "./..." {
		t.Fatalf("generate.patterns = %#v", got)
	}
	if document.Tasks["checksum"].Type != TaskTypeExec {
		t.Fatalf("task type = %q", document.Tasks["checksum"].Type)
	}
}

func TestLoadFile_whenEncodingOrLineEndingIsInvalid_shouldReject(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    string
	}{
		{name: "bom", content: append([]byte{0xef, 0xbb, 0xbf}, []byte("version = 1\n")...), want: "BOM"},
		{name: "crlf", content: []byte("version = 1\r\n"), want: "LF"},
		{name: "invalid utf8", content: []byte{'v', 'e', 'r', 's', 'i', 'o', 'n', ' ', '=', ' ', 0xff, '\n'}, want: "UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), FileName)
			if err := os.WriteFile(path, tt.content, 0o600); err != nil {
				t.Fatalf("写入测试文件失败: %v", err)
			}
			_, err := LoadFile(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("错误 = %v, want contains %q", err, tt.want)
			}
		})
	}
}

func TestLoadFile_whenStructureIsInvalid_shouldReject(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "missing version", content: "[project]\nname = \"demo\"\n", want: "version"},
		{name: "future version", content: "version = 2\n", want: "不支持"},
		{name: "unknown field", content: "version = 1\nunknown = true\n", want: "未知字段"},
		{name: "duplicate field", content: "version = 1\nversion = 1\n", want: "重复"},
		{name: "invalid timeout", content: "version = 1\n[execution]\ndefault-timeout = \"soon\"\n", want: "duration"},
		{name: "invalid parallelism", content: "version = 1\n[execution]\nmax-parallel = 0\n", want: "max-parallel"},
		{name: "unknown task type", content: "version = 1\n[tasks.bad]\ntype = \"shell\"\n", want: "任务类型"},
		{name: "missing dependency", content: "version = 1\n[tasks.one]\ntype = \"group\"\ndepends-on = [\"missing\"]\n", want: "missing"},
		{name: "missing tool", content: "version = 1\n[tasks.one]\ntype = \"exec\"\ntool = \"missing\"\n", want: "missing"},
		{name: "cached task missing inputs", content: "version = 1\n[tasks.one]\ntype = \"go\"\nargs = [\"list\", \"./...\"]\noutputs = [\"build/out\"]\ncache = true\n", want: "inputs"},
		{name: "cached task missing outputs", content: "version = 1\n[tasks.one]\ntype = \"go\"\nargs = [\"list\", \"./...\"]\ninputs = [\"**/*.go\"]\ncache = true\n", want: "outputs"},
		{name: "unknown command task", content: "version = 1\n[commands.build]\nbefore = [\"missing\"]\n", want: "missing"},
		{name: "task output escapes", content: "version = 1\n[tasks.one]\ntype = \"delete\"\noutputs = [\"../outside\"]\n", want: "项目根目录"},
		{name: "delete task cache", content: "version = 1\n[tasks.one]\ntype = \"delete\"\ninputs = [\"input\"]\noutputs = [\"output\"]\ncache = true\n", want: "不能启用 cache"},
		{name: "invalid profile name", content: "version = 1\n[profiles.\"bad name\"]\ngo-args = []\n", want: "Profile 名称"},
		{name: "invalid task environment", content: "version = 1\n[tasks.one]\ntype = \"go\"\nargs = [\"version\"]\n[tasks.one.environment]\n\"BAD-NAME\" = \"value\"\n", want: "environment 名称"},
		{name: "invalid command environment", content: "version = 1\n[commands.build.environment]\n\"BAD-NAME\" = \"value\"\n", want: "environment 名称"},
		{name: "invalid profile environment", content: "version = 1\n[profiles.dev.environment]\n\"BAD-NAME\" = \"value\"\n", want: "environment 名称"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadFile(writeBuildFile(t, tt.content))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("错误 = %v, want contains %q", err, tt.want)
			}
		})
	}
}

func writeBuildFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写入测试 goark.build 失败: %v", err)
	}
	return path
}
