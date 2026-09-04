package processrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOSRunner_whenProcessExits_shouldPreserveExitCode(t *testing.T) {
	err := OSRunner{}.Run(Request{
		Name: os.Args[0],
		Args: []string{"-test.run=TestProcessHelper", "--", "exit"},
		Env:  append(os.Environ(), "GOARK_PROCESS_HELPER=1"),
		Out:  &bytes.Buffer{},
		Err:  &bytes.Buffer{},
	})
	code, ok := ExitCode(err)
	if !ok || code != 23 {
		t.Fatalf("退出码 = %d, ok=%t, err=%v", code, ok, err)
	}
}

func TestOSRunner_whenContextCanceled_shouldTerminateProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	output := &cancelWriter{cancel: cancel}
	err := OSRunner{}.Run(Request{
		Context: ctx,
		Name:    os.Args[0],
		Args:    []string{"-test.run=TestProcessHelper", "--", "wait"},
		Env:     append(os.Environ(), "GOARK_PROCESS_HELPER=1"),
		Out:     output,
		Err:     &bytes.Buffer{},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("错误 = %v", err)
	}
}

type cancelWriter struct {
	once   sync.Once
	cancel context.CancelFunc
}

func (w *cancelWriter) Write(data []byte) (int, error) {
	w.once.Do(w.cancel)
	return len(data), nil
}

func TestProcessHelper(t *testing.T) {
	if os.Getenv("GOARK_PROCESS_HELPER") != "1" {
		return
	}
	separator := -1
	for index, value := range os.Args {
		if value == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(24)
	}
	switch strings.TrimSpace(os.Args[separator+1]) {
	case "exit":
		os.Exit(23)
	case "wait":
		_, _ = fmt.Fprintln(os.Stdout, "ready")
		time.Sleep(time.Hour)
		os.Exit(25)
	default:
		os.Exit(26)
	}
}
