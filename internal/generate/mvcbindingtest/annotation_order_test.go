package generate_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"goark.dev/cli/internal/generate"
)

func TestRouteAnnotationOrderDoesNotChangeBehavior(t *testing.T) {
	annotations := []string{
		"//goark-web:post(\"/users\")\n",
		"//goark-web:request-body[input]\n",
		"//goark-web:validated(\"create\")\n",
		"//goark-web:response-body\n",
	}
	var expected []byte
	for shift := range annotations {
		dir := t.TempDir()
		source := "package app\ntype Input struct { Name string }\n" +
			"//goark-web:controller\ntype Controller struct{}\n"
		for index := range annotations {
			source += annotations[(index+shift)%len(annotations)]
		}
		source += "func (*Controller) Save(input Input) string { return input.Name }\n"
		if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		data, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir})
		if err != nil {
			t.Fatal(err)
		}
		if shift == 0 {
			expected = data
		}
		if !bytes.Equal(data, expected) {
			t.Fatalf("轮换 %d 次后路由生成行为改变", shift)
		}
	}
}

func TestConflictingParameterSelectorsRejected(t *testing.T) {
	dir := t.TempDir()
	source := `package app
//goark-web:controller
type Controller struct{}
//goark-web:get("/find")
//goark-web:request-param[left](param="right")
//goark-web:request-param[right]
func (*Controller) Find(left string, right string) string { return left + right }
`
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := generate.GenerateAnnotations(generate.AnnotationScanSpec{Dir: dir}); err == nil {
		t.Fatal("相互冲突的参数选择器不应被静默忽略")
	}
}
