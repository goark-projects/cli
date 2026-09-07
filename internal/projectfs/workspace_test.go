package projectfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceActive(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.27.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		setting string
		want    bool
	}{{"", true}, {"auto", true}, {"off", false}, {"/explicit/go.work", true}} {
		if got := WorkspaceActive(filepath.Join(root, "child"), test.setting); got != test.want {
			t.Fatalf("GOWORK=%q: got %v, want %v", test.setting, got, test.want)
		}
	}
}
