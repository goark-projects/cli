package generate

import "sort"

// ImportSpec 描述生成文件所需的额外导入。
type ImportSpec struct {
	Alias string
	Path  string
}

const (
	ScopeSingleton = "singleton"
	ScopePrototype = "prototype"
)

func sortImports(imports []ImportSpec) {
	sort.SliceStable(imports, func(left int, right int) bool {
		if imports[left].Path == imports[right].Path {
			return imports[left].Alias < imports[right].Alias
		}
		return imports[left].Path < imports[right].Path
	})
}
