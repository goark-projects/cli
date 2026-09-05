package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommand_whenCommandHelpRequested_shouldPrintCommandSpecificHelp(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{command: "run", want: "goark run"},
		{command: "build", want: "goark build"},
		{command: "generate", want: "goark generate"},
		{command: "go", want: "goark go <go-arguments>"},
		{command: "info", want: "goark info"},
		{command: "completion", want: "goark completion"},
	}

	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			command := Command{Out: &stdout, Err: &stderr, Runner: &recordingProcessRunner{}}

			if code := command.Run([]string{"help", test.command}); code != 0 {
				t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("帮助缺少 %q:\n%s", test.want, stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestCommand_whenLifecycleHelpRequested_shouldDescribeCurrentControlFlags(t *testing.T) {
	var stdout bytes.Buffer
	command := Command{Out: &stdout, Err: &bytes.Buffer{}, Runner: &recordingProcessRunner{}}
	if code := command.Run([]string{"help", "build"}); code != 0 {
		t.Fatalf("退出码 = %d", code)
	}
	output := stdout.String()
	for _, flag := range []string{"--goark-profile", "--goark-dry-run", "--goark-offline", "--goark-locked", "--goark-env"} {
		if !strings.Contains(output, flag) {
			t.Fatalf("帮助缺少 %s:\n%s", flag, output)
		}
	}
	for _, removed := range []string{"--goark-no-generate", "--goark-generate-only"} {
		if strings.Contains(output, removed) {
			t.Fatalf("帮助仍包含已删除参数 %s:\n%s", removed, output)
		}
	}
}

func TestCommand_whenGenerateHelpFlagRequested_shouldPrintProjectGenerationHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := Command{Out: &stdout, Err: &stderr, Runner: &recordingProcessRunner{}}

	if code := command.Run([]string{"generate", "--help"}); code != 0 {
		t.Fatalf("退出码 = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "goark generate [package-patterns]") {
		t.Fatalf("帮助输出错误:\n%s", stdout.String())
	}
}

func TestCommand_whenLegacyGeneratorEntryRequested_shouldReject(t *testing.T) {
	for _, args := range [][]string{{"gen"}, {"generate", "annotations"}} {
		var stderr bytes.Buffer
		command := Command{Out: &bytes.Buffer{}, Err: &stderr, Runner: &recordingProcessRunner{}}

		if code := command.Run(args); code != 2 {
			t.Fatalf("参数 %#v 的退出码 = %d", args, code)
		}
	}
}

func TestCommand_whenHelpTopicHasUnexpectedNestedValue_shouldReject(t *testing.T) {
	var stderr bytes.Buffer
	command := Command{Out: &bytes.Buffer{}, Err: &stderr}

	if code := command.Run([]string{"help", "run", "unexpected"}); code != 2 {
		t.Fatalf("退出码 = %d", code)
	}
	if !strings.Contains(stderr.String(), "未知帮助主题") {
		t.Fatalf("错误信息不完整: %q", stderr.String())
	}
}

func TestCommand_whenVersionHasUnexpectedArgument_shouldReject(t *testing.T) {
	var stderr bytes.Buffer
	command := Command{Out: &bytes.Buffer{}, Err: &stderr}

	if code := command.Run([]string{"version", "unexpected"}); code != 2 {
		t.Fatalf("退出码 = %d", code)
	}
	if !strings.Contains(stderr.String(), "不接受参数") {
		t.Fatalf("错误信息不完整: %q", stderr.String())
	}
}
