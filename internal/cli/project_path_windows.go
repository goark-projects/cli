//go:build windows

package cli

import (
	"path/filepath"
	"strings"
)

func samePath(left string, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
