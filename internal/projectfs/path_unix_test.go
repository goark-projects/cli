//go:build !windows

package projectfs

import "testing"

func TestSamePath_whenUnixPathsDifferOnlyByCase_shouldNotMatch(t *testing.T) {
	if SamePath("/tmp/Goark", "/tmp/goark") {
		t.Fatal("Unix 路径比较必须区分大小写")
	}
}

func TestSamePath_whenUnixPathsNormalizeToSamePath_shouldMatch(t *testing.T) {
	if !SamePath("/tmp/goark/../goark", "/tmp/goark") {
		t.Fatal("Unix 路径比较必须先清理路径")
	}
}
