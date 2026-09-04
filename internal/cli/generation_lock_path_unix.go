//go:build !windows

package cli

import "path/filepath"

func normalizeGenerationLockRoot(root string) string {
	return filepath.Clean(root)
}
