package clitest

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestTestArgumentsCannotOverrideReadonly(t *testing.T) {
	for _, separator := range []string{"-args", "--"} {
		t.Run(separator, func(t *testing.T) {
			root := argumentBoundaryProject(t)
			before, err := os.ReadFile(filepath.Join(root, "go.mod"))
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			command := testOSCommand(root, io.Discard, &stderr)
			command.Env = append(command.Env, "GOENV="+filepath.Join(root, "goenv"),
				"GOPROXY=off", "GOTOOLCHAIN=local")
			code := command.Run([]string{"test", "./...", separator, "-mod=mod"})
			after, err := os.ReadFile(filepath.Join(root, "go.mod"))
			if err != nil || !bytes.Equal(before, after) || code == 0 {
				t.Fatalf("业务参数越过只读边界: code=%d, err=%v\n%s\n%s", code, err, after, &stderr)
			}
		})
	}
}

func TestTestArgumentsRemainOpaque(t *testing.T) {
	for _, separator := range []string{"-args", "--"} {
		t.Run(separator, func(t *testing.T) {
			root := argumentBoundaryProject(t)
			var stderr bytes.Buffer
			command := testOSCommand(root, io.Discard, &stderr)
			command.Env = append(command.Env, "GOENV=off", "GOPROXY=off", "GOTOOLCHAIN=local")
			args := []string{"test", "./...", separator, "-mod=mod", "-C=business-value",
				"-tags=business", "-overlay=missing.json", "-modfile=missing.mod"}
			if code := command.Run(args); code != 0 {
				t.Fatalf("业务参数被重新解释: code=%d\n%s", code, &stderr)
			}
		})
	}
}

func argumentBoundaryProject(t *testing.T) string {
	return writeTestModule(t, map[string]string{
		"go.mod": "module example.com/boundary\n\ngo 1.27.0\n\n" +
			"replace example.com/dependency => ./dependency\n",
		"app.go": "package boundary\nimport _ \"example.com/dependency\"\n",
		"app_test.go": `package boundary
import (
    "flag"
    "reflect"
    "testing"
)
var mode = flag.String("mod", "", "业务模式")
var directory = flag.String("C", "", "业务目录")
var tags = flag.String("tags", "", "业务标签")
var overlay = flag.String("overlay", "", "业务覆盖配置")
var modfile = flag.String("modfile", "", "业务模块文件")
func TestBusinessArguments(t *testing.T) {
    if reflect.DeepEqual(flag.Args(), []string{"-mod=mod", "-C=business-value",
        "-tags=business", "-overlay=missing.json", "-modfile=missing.mod"}) {
        return
    }
    if *mode != "mod" || *directory != "business-value" || *tags != "business" ||
        *overlay != "missing.json" || *modfile != "missing.mod" {
        t.Fatalf("业务参数未完整透传")
    }
}
`,
		"dependency/go.mod": "module example.com/dependency\n\ngo 1.27.0\n",
		"dependency/dep.go": "package dependency\n",
		"goenv":             "GOFLAGS=-mod=readonly\n",
	})
}
