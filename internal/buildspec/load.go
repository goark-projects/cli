package buildspec

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"time"
	"unicode/utf8"

	toml "github.com/pelletier/go-toml/v2"
)

// LoadFile 严格读取并校验一个 goark.build 文件。
func LoadFile(path string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("读取 %s 失败: %w", FileName, err)
	}
	if err := validateEncoding(data); err != nil {
		return Document{}, fmt.Errorf("%s 格式无效: %w", FileName, err)
	}

	document := defaultDocument()
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Document{}, classifyDecodeError(err)
	}
	if err := validateDocument(document); err != nil {
		return Document{}, fmt.Errorf("%s 校验失败: %w", FileName, err)
	}
	return document, nil
}

func defaultDocument() Document {
	return Document{
		Execution: Execution{
			MaxParallel:    runtime.GOMAXPROCS(0),
			FailFast:       true,
			DefaultTimeout: Duration{Duration: 5 * time.Minute},
		},
		Generate: Generate{
			Patterns:   []string{"./..."},
			CleanStale: true,
		},
		Commands: make(map[string]Command),
		Tools:    make(map[string]Tool),
		Tasks:    make(map[string]Task),
		Profiles: make(map[string]Profile),
	}
}

func validateEncoding(data []byte) error {
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return fmt.Errorf("必须使用 UTF-8 无 BOM 编码")
	}
	if !utf8.Valid(data) {
		return fmt.Errorf("必须使用有效 UTF-8 编码")
	}
	if bytes.ContainsRune(data, '\r') {
		return fmt.Errorf("必须使用 LF 换行，禁止 CRLF 或 CR")
	}
	return nil
}

func classifyDecodeError(err error) error {
	message := err.Error()
	switch {
	case bytes.Contains([]byte(message), []byte("strict mode")):
		return fmt.Errorf("%s 包含未知字段: %w", FileName, err)
	case bytes.Contains([]byte(message), []byte("already defined")),
		bytes.Contains([]byte(message), []byte("cannot redefine")):
		return fmt.Errorf("%s 包含重复字段: %w", FileName, err)
	default:
		return fmt.Errorf("解析 %s 失败: %w", FileName, err)
	}
}
