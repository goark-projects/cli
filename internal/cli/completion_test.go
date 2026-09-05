package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommand_whenCompletionRequested_shouldGenerateSupportedShellScript(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{shell: "bash", want: "complete -F _goark goark"},
		{shell: "zsh", want: "#compdef goark"},
		{shell: "fish", want: "complete -c goark"},
		{shell: "powershell", want: "Register-ArgumentCompleter -Native -CommandName goark"},
	}

	for _, test := range tests {
		t.Run(test.shell, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			command := Command{Out: &stdout, Err: &stderr}

			if code := command.Run([]string{"completion", test.shell}); code != 0 {
				t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), test.want) || !strings.Contains(stdout.String(), "generate") {
				t.Fatalf("%s 补全脚本不完整:\n%s", test.shell, stdout.String())
			}
		})
	}
}

func TestCommand_whenCompletionShellUnsupported_shouldReturnUsageError(t *testing.T) {
	var stderr bytes.Buffer
	command := Command{Out: &bytes.Buffer{}, Err: &stderr}

	if code := command.Run([]string{"completion", "cmd"}); code != 2 {
		t.Fatalf("退出码 = %d", code)
	}
	if !strings.Contains(stderr.String(), "bash、zsh、fish、powershell") {
		t.Fatalf("错误信息不完整: %q", stderr.String())
	}
}

func TestCommand_whenCompletionRequested_shouldIncludeNestedCommands(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		var stdout bytes.Buffer
		command := Command{Out: &stdout, Err: &bytes.Buffer{}}
		if code := command.Run([]string{"completion", shell}); code != 0 {
			t.Fatalf("%s 退出码 = %d", shell, code)
		}
		for _, fragment := range []string{"configuration", "registry", "annotations", "app"} {
			if !strings.Contains(stdout.String(), fragment) {
				t.Fatalf("%s 补全缺少 %q", shell, fragment)
			}
		}
		for _, commandName := range []string{"clean", "tasks", "task", "graph", "sync", "tools", "tool"} {
			if !strings.Contains(stdout.String(), commandName) {
				t.Fatalf("%s 补全缺少命令 %q", shell, commandName)
			}
		}
	}
}
