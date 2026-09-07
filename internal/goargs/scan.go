// Package goargs 统一 Go 构建参数与业务透传参数的词法边界。
package goargs

import (
	"iter"
	"strings"
)

// Entry 是一个完整参数；透传区作为整体返回，调用方不得继续解析。
type Entry struct {
	Name        string
	Value       string
	HasValue    bool
	Passthrough bool
	Args        []string
}

// Scan 保留参数顺序和值边界；返回的切片只读且引用原始参数。
func Scan(args []string) iter.Seq[Entry] {
	return func(yield func(Entry) bool) {
		for index := 0; index < len(args); index++ {
			start, arg := index, args[index]
			if arg == "--" || arg == "-args" || arg == "--args" {
				yield(Entry{Passthrough: true, Args: args[index:]})
				return
			}
			entry := Entry{Value: arg}
			if strings.HasPrefix(arg, "-") && arg != "-" {
				entry.Name, entry.Value, entry.HasValue = strings.Cut(arg, "=")
				if strings.HasPrefix(entry.Name, "--") {
					entry.Name = strings.TrimPrefix(entry.Name, "-")
				}
				if !entry.HasValue && consumesValue(entry.Name) && index+1 < len(args) {
					index++
					entry.Value, entry.HasValue = args[index], true
				}
			}
			entry.Args = args[start : index+1]
			if !yield(entry) {
				return
			}
		}
	}
}

// BuildFlagConsumesValue 判断构建参数是否从后一参数读取值。
func BuildFlagConsumesValue(arg string) bool {
	if strings.Contains(arg, "=") {
		return false
	}
	_, ok := buildValueFlags[arg]
	return ok
}

func consumesValue(name string) bool {
	if BuildFlagConsumesValue(name) {
		return true
	}
	if strings.HasPrefix(name, "-test.") {
		name = "-" + strings.TrimPrefix(name, "-test.")
	}
	_, ok := testValueFlags[name]
	return ok
}

var buildValueFlags = map[string]struct{}{
	"-C": {}, "-asmflags": {}, "-buildmode": {}, "-buildvcs": {}, "-compiler": {},
	"-covermode": {}, "-coverpkg": {}, "-exec": {}, "-gccgoflags": {}, "-gcflags": {},
	"-installsuffix": {}, "-ldflags": {}, "-mod": {}, "-modfile": {}, "-o": {},
	"-overlay": {}, "-p": {}, "-pkgdir": {}, "-pgo": {}, "-tags": {}, "-toolexec": {},
}

var testValueFlags = map[string]struct{}{
	"-bench": {}, "-benchtime": {}, "-blockprofile": {}, "-blockprofilerate": {},
	"-count": {}, "-coverprofile": {}, "-cpu": {}, "-cpuprofile": {}, "-fuzz": {},
	"-fuzzminimizetime": {}, "-fuzztime": {}, "-list": {}, "-memprofile": {},
	"-memprofilerate": {}, "-mutexprofile": {}, "-mutexprofilefraction": {},
	"-outputdir": {}, "-parallel": {}, "-run": {}, "-shuffle": {}, "-skip": {},
	"-timeout": {}, "-trace": {}, "-vet": {},
}
