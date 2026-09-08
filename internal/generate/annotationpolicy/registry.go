package annotationpolicy

import (
	"fmt"
	"strings"

	"goark.dev/cli/internal/generate/annotationparse"
)

// ExtensionName 生成跨平台唯一且不含路径的扩展标识。
func ExtensionName(raw string, index int, seen map[string]bool) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		name = fmt.Sprintf("extension_%02d", index+1)
	}
	for _, char := range name {
		if char != '_' && char != '-' && !(char >= 'a' && char <= 'z') &&
			!(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') {
			return "", fmt.Errorf("invalid annotation extension name %q", name)
		}
	}
	key := strings.ToLower(name)
	if seen[key] {
		return "", fmt.Errorf("duplicate annotation extension name %q", name)
	}
	seen[key] = true
	return name, nil
}

// RegisterDescriptors 同步规范化注册键和分发描述符，避免空白名称导致绑定丢失。
func RegisterDescriptors[T comparable, C any](
	items []Descriptor[T, C], registry map[string]Descriptor[T, C],
	namespaces *annotationparse.Namespaces,
) ([]Descriptor[T, C], error) {
	out := append([]Descriptor[T, C](nil), items...)
	for index := range out {
		descriptor := &out[index]
		descriptor.Name = strings.TrimSpace(descriptor.Name)
		if descriptor.Name == "" {
			return nil, fmt.Errorf("annotation descriptor name is required")
		}
		if err := namespaces.Register(descriptor.Name); err != nil {
			return nil, err
		}
		if _, exists := registry[descriptor.Name]; exists {
			return nil, fmt.Errorf("duplicate annotation descriptor %q", descriptor.Name)
		}
		registry[descriptor.Name] = *descriptor
	}
	return out, nil
}
