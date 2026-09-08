// Package genpipeline 提供确定顺序、失败即停的生成阶段执行器。
package genpipeline

import "fmt"

// Step 声明一个阶段；阶段仅访问调用方持有的本次生成状态。
type Step struct {
	Name string
	Run  func() error
}

// Execute 先校验完整阶段链，再按声明顺序执行，保留原始错误。
func Execute(steps ...Step) error {
	seen := make(map[string]bool, len(steps))
	for _, step := range steps {
		if step.Name == "" || step.Run == nil || seen[step.Name] {
			return fmt.Errorf("invalid generation stage %q", step.Name)
		}
		seen[step.Name] = true
	}
	for _, step := range steps {
		if err := step.Run(); err != nil {
			return fmt.Errorf("%s: %w", step.Name, err)
		}
	}
	return nil
}
