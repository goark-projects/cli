//go:build !windows

package cli

import "path/filepath"

func samePath(left string, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
