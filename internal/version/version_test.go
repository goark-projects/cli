package version

import "testing"

func TestResolve_whenBuildVersionProvided_shouldPreferBuildVersion(t *testing.T) {
	if got := resolve("v0.0.1", "v9.9.9"); got != "0.0.1" {
		t.Fatalf("版本 = %q", got)
	}
}

func TestResolve_whenInstalledFromModule_shouldUseModuleVersion(t *testing.T) {
	if got := resolve("", "v0.0.1"); got != "0.0.1" {
		t.Fatalf("版本 = %q", got)
	}
}

func TestResolve_whenBuiltLocally_shouldReturnDevelopmentVersion(t *testing.T) {
	for _, moduleVersion := range []string{"", "(devel)"} {
		if got := resolve("", moduleVersion); got != Development {
			t.Fatalf("模块版本 %q 解析为 %q", moduleVersion, got)
		}
	}
}
