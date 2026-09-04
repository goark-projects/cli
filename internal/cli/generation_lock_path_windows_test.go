//go:build windows

package cli

import "testing"

func TestNormalizeGenerationLockRoot_whenPathCaseDiffers_shouldReturnSameIdentity(t *testing.T) {
	left := normalizeGenerationLockRoot(`C:\Work\Goark`)
	right := normalizeGenerationLockRoot(`c:\work\goark`)
	if left != right {
		t.Fatalf("锁路径身份不一致: %q != %q", left, right)
	}
}
