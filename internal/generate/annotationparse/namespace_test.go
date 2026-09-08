package annotationparse

import (
	"go/ast"
	"testing"
)

func TestParseCommentsNamespaces(t *testing.T) {
	group := &ast.CommentGroup{List: []*ast.Comment{
		{Text: `//goark:service("users")`},
		{Text: `//goark-web:delete("/users")`},
		{Text: `//goark-orm:delete("statement")`},
		{Text: `//goark-web:request-param[id]("userId")`},
	}}
	items, err := ParseComments(group)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"service", "goark-web:delete", "goark-orm:delete", "goark-web:request-param"}
	if len(items) != len(want) {
		t.Fatalf("注解数量 = %d，期望 %d", len(items), len(want))
	}
	for i, name := range want {
		if items[i].Name != name {
			t.Errorf("注解名称 = %q，期望 %q", items[i].Name, name)
		}
	}
	if items[3].Selector != "id" || items[3].Values[0].Text() != "userId" {
		t.Fatalf("参数选择器或值丢失: %+v", items[3])
	}
}

func TestParseCommentsIgnoresOrdinaryComments(t *testing.T) {
	for _, value := range []string{
		"// mentions goark-web:get", "/*goark-web:get*/", "//goark-web docs",
	} {
		items, err := ParseComments(&ast.CommentGroup{List: []*ast.Comment{{Text: value}}})
		if err != nil || len(items) != 0 {
			t.Fatalf("普通注释被解析为注解: %q, %v, %v", value, items, err)
		}
	}
}

func TestParseCommentsRejectsEmptyDomainName(t *testing.T) {
	for _, value := range []string{"//goark-web:", "//goark-web:()", "//goark-web:[id]"} {
		_, err := ParseComments(&ast.CommentGroup{List: []*ast.Comment{{Text: value}}})
		if err == nil {
			t.Fatalf("空领域注解名称未被拒绝: %q", value)
		}
	}
}

func TestParseCommentsLeavesForeignSyntaxToItsOwner(t *testing.T) {
	group := &ast.CommentGroup{List: []*ast.Comment{{Text: "//goark-orm:foreign(unclosed"}}}
	items, err := ParseComments(group, "goark", "goark-web")
	if err != nil || len(items) != 0 {
		t.Fatalf("不应解析其他生成器的语法: %v, %v", items, err)
	}
}

func TestParseCommentsRejectsNamespaceBypass(t *testing.T) {
	for _, value := range []string{"//goark:goark-web:get", "//goark-web:orm:delete"} {
		_, err := ParseComments(&ast.CommentGroup{List: []*ast.Comment{{Text: value}}})
		if err == nil {
			t.Fatalf("嵌套命名空间未被拒绝: %q", value)
		}
	}
}
