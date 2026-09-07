package buildplan

import (
	"reflect"
	"testing"
)

func TestControlArgumentsPreserveGoFlagValues(t *testing.T) {
	for _, args := range [][]string{
		{"-ldflags", "--goark-offline"},
		{"-run", "--goark-dry-run"},
		{"./...", "-args", "--goark-offline", "--goark-profile=missing"},
		{"./...", "--", "--goark-offline"},
	} {
		got, control, err := ParseControlArguments(args)
		if err != nil || control.Offline || control.DryRun || control.Profile != "" ||
			!reflect.DeepEqual(got, args) {
			t.Fatalf("业务值影响 Goark 控制参数: args=%q, got=%q, control=%+v, err=%v",
				args, got, control, err)
		}
	}
}
