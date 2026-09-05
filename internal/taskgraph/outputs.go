package taskgraph

import (
	"fmt"
	"path"
	"strings"

	"goark.dev/cli/internal/buildspec"
)

func validateOutputConflicts(tasks map[string]buildspec.Task) error {
	names := sortedTaskNames(tasks)
	for leftIndex, leftName := range names {
		for _, leftOutput := range tasks[leftName].Outputs {
			for _, rightName := range names[leftIndex+1:] {
				for _, rightOutput := range tasks[rightName].Outputs {
					if outputsMayOverlap(leftOutput, rightOutput) {
						return fmt.Errorf("任务 %q 的输出 %q 与任务 %q 的输出 %q 存在输出冲突", leftName, leftOutput, rightName, rightOutput)
					}
				}
			}
		}
	}
	return nil
}

func outputsMayOverlap(left string, right string) bool {
	left = normalizeOutput(left)
	right = normalizeOutput(right)
	if left == right {
		return true
	}
	leftMeta := hasMeta(left)
	rightMeta := hasMeta(right)
	if !leftMeta && !rightMeta {
		return pathWithinOutput(left, right) || pathWithinOutput(right, left)
	}
	leftPrefix := staticPrefix(left)
	rightPrefix := staticPrefix(right)
	if patternsProvablyDisjoint(left, right) {
		return false
	}
	return leftPrefix == rightPrefix || pathWithinOutput(leftPrefix, rightPrefix) || pathWithinOutput(rightPrefix, leftPrefix)
}

func patternsProvablyDisjoint(left string, right string) bool {
	leftSegments := strings.Split(left, "/")
	rightSegments := strings.Split(right, "/")
	if len(leftSegments) != len(rightSegments) {
		return false
	}
	for index, leftSegment := range leftSegments {
		rightSegment := rightSegments[index]
		if leftSegment == "**" || rightSegment == "**" {
			return false
		}
		leftMeta := hasMeta(leftSegment)
		rightMeta := hasMeta(rightSegment)
		switch {
		case !leftMeta && !rightMeta && leftSegment != rightSegment:
			return true
		case !leftMeta && rightMeta:
			matched, err := path.Match(rightSegment, leftSegment)
			if err == nil && !matched {
				return true
			}
		case leftMeta && !rightMeta:
			matched, err := path.Match(leftSegment, rightSegment)
			if err == nil && !matched {
				return true
			}
		}
	}
	return false
}

func normalizeOutput(value string) string {
	value = strings.TrimPrefix(filepathSlash(value), "./")
	return path.Clean(value)
}

func filepathSlash(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}

func hasMeta(value string) bool {
	return strings.ContainsAny(value, "*?[")
}

func staticPrefix(value string) string {
	index := strings.IndexAny(value, "*?[")
	if index < 0 {
		return value
	}
	prefix := strings.TrimSuffix(value[:index], "/")
	if prefix == "" {
		return "."
	}
	return prefix
}

func pathWithinOutput(root string, target string) bool {
	if root == "." {
		return true
	}
	return target == root || strings.HasPrefix(target, root+"/")
}
