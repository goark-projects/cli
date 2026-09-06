package mvcrouting

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestSupportedMethod(t *testing.T) {
	t.Parallel()
	for _, method := range []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
		http.MethodTrace,
	} {
		if !SupportedMethod(method) {
			t.Fatalf("SupportedMethod(%q) = false", method)
		}
	}
	if SupportedMethod(http.MethodConnect) {
		t.Fatal("SupportedMethod(CONNECT) = true")
	}
}

func TestDefaultStatusAndConstructor(t *testing.T) {
	t.Parallel()
	if got := DefaultStatus([]string{http.MethodPost}); got != http.StatusCreated {
		t.Fatalf("DefaultStatus(POST) = %d", got)
	}
	if got := DefaultStatus([]string{http.MethodGet}); got != http.StatusOK {
		t.Fatalf("DefaultStatus(GET) = %d", got)
	}
	if got := Constructor(http.MethodPatch); got != "PATCH" {
		t.Fatalf("Constructor(PATCH) = %q", got)
	}
	if got := Constructor(http.MethodConnect); got != "Handle" {
		t.Fatalf("Constructor(CONNECT) = %q", got)
	}
}

func TestNormalizePaths(t *testing.T) {
	t.Parallel()
	got := NormalizePaths([]string{"users/", "/users", "", "/"})
	want := []string{"/users", "/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizePaths() = %#v, want %#v", got, want)
	}
}

func TestCombineMethods(t *testing.T) {
	t.Parallel()
	got := CombineMethods(
		[]string{http.MethodGet, http.MethodPost},
		[]string{http.MethodPost, http.MethodPatch},
		true,
	)
	want := []string{http.MethodPost, http.MethodPatch, http.MethodGet}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CombineMethods() = %#v, want %#v", got, want)
	}

	inherited := CombineMethods([]string{http.MethodGet}, nil, false)
	if !reflect.DeepEqual(inherited, []string{http.MethodGet}) {
		t.Fatalf("CombineMethods() inherited = %#v", inherited)
	}
}

func TestJoinPaths(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"/api|users/": "/api/users",
		"/|users":     "/users",
		"api|/":       "/api",
	}
	for input, want := range cases {
		base, path, ok := strings.Cut(input, "|")
		if !ok {
			t.Fatalf("invalid test input %q", input)
		}
		got := JoinPaths(base, path)
		if got != want {
			t.Fatalf("JoinPaths(%q) = %q, want %q", input, got, want)
		}
	}
}
