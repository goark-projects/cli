//go:build windows

package projectlock

import "testing"

func TestNormalizeGenerationLockRoot_whenPathCaseDiffers_shouldReturnSameIdentity(t *testing.T) {
	left := NormalizeRoot(`C:\Work\Goark`)
	right := NormalizeRoot(`c:\work\goark`)
	if left != right {
		t.Fatalf("锁路径身份不一致: %q != %q", left, right)
	}
}
