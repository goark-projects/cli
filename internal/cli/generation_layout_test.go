package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateProject_whenMultipleResponsibilitiesExist_shouldSplitFiles(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.26.0\n",
		"app/app.go": `package app

import arkweb "goark.dev/arkarta/web"

//goark:service
type UserService struct{}

//goark:web-interceptor
type AuditInterceptor struct{}

//goark:controller
type UserController struct{}

//goark:get("/users")
func (*UserController) Users(*arkweb.Context) error { return nil }
`,
	})
	project, err := newTestProjectResolver(root).Resolve()
	if err != nil {
		t.Fatalf("发现项目失败: %v", err)
	}
	results, err := generateProject(project, false)
	if err != nil {
		t.Fatalf("生成项目失败: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("生成文件数量 = %d，期望 3: %#v", len(results), results)
	}
	for _, name := range []string{
		"zz_goark_core_gen.go",
		"zz_goark_web_gen.go",
		"zz_goark_mvc_gen.go",
	} {
		path := filepath.Join(root, "app", "gen", name)
		data, readErr := os.ReadFile(path)
		if readErr != nil || !strings.Contains(string(data), "package gen") {
			t.Fatalf("职责生成文件 %s 无效: %v\n%s", name, readErr, data)
		}
	}
}

func TestGenerateProject_whenInjectionFieldIsPrivate_shouldReject(t *testing.T) {
	root := writeTestModule(t, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.26.0\n",
		"app/app.go": `package app

//goark:service
type UserService struct {
	//goark:autowired
	repository Repository
}

type Repository interface{}
`,
	})
	project, err := newTestProjectResolver(root).Resolve()
	if err != nil {
		t.Fatalf("发现项目失败: %v", err)
	}
	_, err = generateProject(project, false)
	if err == nil || !strings.Contains(err.Error(), "要求注入字段 UserService.repository 可导出") {
		t.Fatalf("未导出注入字段应失败: %v", err)
	}
}
