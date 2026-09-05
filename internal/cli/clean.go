package cli

import (
	"fmt"
	"io"

	"goark.dev/cli/internal/projectclean"
	"goark.dev/cli/internal/projectlock"
)

func (c Command) runClean(args []string) int {
	if isHelpOnly(args) {
		c.printCleanHelp(c.Out)
		return 0
	}
	dryRun := false
	for _, argument := range args {
		if argument != "--goark-dry-run" || dryRun {
			_, _ = fmt.Fprintf(c.Err, "goark clean 仅支持 --goark-dry-run: %s\n", argument)
			return 2
		}
		dryRun = true
	}
	project, err := c.resolveProjectMetadata(c.Dir)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return 2
	}
	clean := func() int {
		removed, err := (projectclean.Cleaner{Root: project.Root, Document: project.Build}).Run(dryRun)
		if err != nil {
			_, _ = fmt.Fprintln(c.Err, err)
			return 1
		}
		for _, path := range removed {
			if dryRun {
				_, _ = fmt.Fprintf(c.Err, "would remove %s\n", path)
			} else {
				_, _ = fmt.Fprintf(c.Err, "removed %s\n", path)
			}
		}
		return 0
	}
	if dryRun {
		return clean()
	}
	lock, err := projectlock.Acquire(c.Context, project.Root)
	if err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		return executionErrorCode(err)
	}
	code := clean()
	if err := lock.Release(); err != nil {
		_, _ = fmt.Fprintln(c.Err, err)
		if code == 0 {
			return 1
		}
	}
	return code
}

func (c Command) printCleanHelp(writer io.Writer) {
	_, _ = fmt.Fprint(writer, "Usage:\n  goark clean [--goark-dry-run]\n")
}
