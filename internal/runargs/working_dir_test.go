package runargs

import (
	"path/filepath"
	"testing"
)

func TestWorkingDirectoryIgnoresPassthroughAndValues(t *testing.T) {
	base := t.TempDir()
	for _, args := range [][]string{
		{"./...", "-args", "-C=business"},
		{"./...", "--", "-C", "business"},
		{"-ldflags", "-C=business"},
		{"-run", "-C=business"},
		{"-test.run", "-C=business"},
	} {
		got, err := EffectiveWorkingDir(base, args)
		if err != nil || got != base {
			t.Fatalf("业务值影响工作目录: args=%q, got=%q, err=%v", args, got, err)
		}
	}
	got, err := EffectiveWorkingDir(base, []string{"-C", "real", "-args", "-C=business"})
	if err != nil || got != filepath.Join(base, "real") {
		t.Fatalf("真正的 -C 未生效: got=%q, err=%v", got, err)
	}
	for _, args := range [][]string{{"-C"}, {"-C="}, {"-C", ""}} {
		if _, err := EffectiveWorkingDir(base, args); err == nil {
			t.Fatalf("未拒绝空目录: %q", args)
		}
	}
}
