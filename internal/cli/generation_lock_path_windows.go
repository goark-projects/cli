//go:build windows

package cli

import (
	"path/filepath"
	"strings"
)

func normalizeGenerationLockRoot(root string) string {
	return strings.ToLower(filepath.Clean(root))
}
