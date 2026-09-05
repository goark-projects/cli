package envutil

import (
	"reflect"
	"runtime"
	"testing"
)

func TestEnvironmentOperations_shouldFollowPlatformNameSemantics(t *testing.T) {
	environment := map[string]string{"PATH": "first"}
	Set(environment, "Path", "second")
	value, ok := Lookup(environment, "PATH")
	if runtime.GOOS == "windows" {
		if !ok || value != "second" || !reflect.DeepEqual(environment, map[string]string{"Path": "second"}) {
			t.Fatalf("Windows 环境 = %#v, value=%q, ok=%t", environment, value, ok)
		}
		return
	}
	if !ok || value != "first" || !reflect.DeepEqual(environment, map[string]string{"PATH": "first", "Path": "second"}) {
		t.Fatalf("Unix 环境 = %#v, value=%q, ok=%t", environment, value, ok)
	}
}
