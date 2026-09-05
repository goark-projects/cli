package taskcache

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"goark.dev/cli/internal/atomicfile"
)

const manifestVersion = 1

// Store 管理项目内的任务缓存清单。
type Store struct {
	root string
}

type manifest struct {
	Version     int          `json:"version"`
	Fingerprint string       `json:"fingerprint"`
	Outputs     []fileDigest `json:"outputs"`
}

// NewStore 创建项目任务缓存存储。
func NewStore(projectRoot string) Store {
	return Store{root: filepath.Join(projectRoot, ".goark", "cache", "tasks")}
}

// Lookup 重新校验输出内容后判断任务是否命中缓存。
func (s Store) Lookup(context Context) (bool, error) {
	fingerprint, err := Fingerprint(context)
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(s.manifestPath(context.TaskName, fingerprint))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cached manifest
	if err := decoder.Decode(&cached); err != nil {
		return false, nil
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return false, nil
	}
	if cached.Version != manifestVersion || cached.Fingerprint != fingerprint {
		return false, nil
	}
	outputs, err := collect(context.Root, context.Task.Outputs, true)
	if err != nil {
		return false, nil
	}
	return reflect.DeepEqual(cached.Outputs, outputs), nil
}

// Save 校验输出后原子保存任务缓存清单。
func (s Store) Save(context Context) error {
	fingerprint, err := Fingerprint(context)
	if err != nil {
		return err
	}
	outputs, err := collect(context.Root, context.Task.Outputs, true)
	if err != nil {
		return err
	}
	data, err := json.Marshal(manifest{Version: manifestVersion, Fingerprint: fingerprint, Outputs: outputs})
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicfile.Write(s.manifestPath(context.TaskName, fingerprint), data, 0o644)
}

func (s Store) manifestPath(taskName string, fingerprint string) string {
	return filepath.Join(s.root, taskName, fingerprint+".json")
}
