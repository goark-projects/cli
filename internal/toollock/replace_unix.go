//go:build !windows

package toollock

import "os"

func replaceFile(source string, target string) error {
	return os.Rename(source, target)
}
