package codegencmd

import "testing"

func TestParseBeanSpec_whenSpecHasOptions_shouldParseBean(t *testing.T) {
	bean, err := parseBeanSpec(
		"userService=NewUserService;deps=userRepository, clock;" +
			"scope=prototype;lazy;primary",
	)
	if err != nil {
		t.Fatalf("parse bean failed: %v", err)
	}
	if bean.Name != "userService" || bean.Provider != "NewUserService" {
		t.Fatalf("unexpected bean identity: %#v", bean)
	}
	if bean.Scope != "prototype" || !bean.Lazy || !bean.Primary {
		t.Fatalf("unexpected bean options: %#v", bean)
	}
	if len(bean.Dependencies) != 2 ||
		bean.Dependencies[0] != "userRepository" ||
		bean.Dependencies[1] != "clock" {
		t.Fatalf("unexpected dependencies: %#v", bean.Dependencies)
	}
}

func TestParseBeanSpec_whenSpecInvalid_shouldReturnError(t *testing.T) {
	cases := []string{
		"",
		"userService",
		"userService=NewUserService;unknown",
		"userService=NewUserService;deps=",
	}
	for _, item := range cases {
		if _, err := parseBeanSpec(item); err == nil {
			t.Fatalf("expected parse error for %q", item)
		}
	}
}
