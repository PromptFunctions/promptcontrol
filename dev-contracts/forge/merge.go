package forge

import (
	"fmt"
	"strings"

	"github.com/PromptFunctions/promptcontrol/dev-contracts/gating"
)

const fileNotFoundPrefix = "file not found on disk: "

type mergeResult struct {
	Applied []string
	Skipped []string
}

func mergeMissingValues(target, source, current map[string]any, missing []string, validation gating.ValidationSpec) mergeResult {
	structured := make([]string, 0, len(missing))
	fileMisses := make([]string, 0, len(missing))
	result := mergeResult{
		Applied: make([]string, 0, len(missing)),
		Skipped: make([]string, 0, len(missing)),
	}
	for _, item := range missing {
		if isStructuredMissingPath(item) {
			structured = append(structured, item)
			continue
		}
		fileMisses = append(fileMisses, item)
	}

	for _, path := range structured {
		if copyJSONPathValue(target, source, path) {
			result.Applied = append(result.Applied, path)
			continue
		}
		result.Skipped = append(result.Skipped, path)
	}

	for _, raw := range fileMisses {
		paths := locateFileItemPaths(current, validation, normalizeMissingFileValue(raw))
		if len(paths) == 0 {
			result.Skipped = append(result.Skipped, raw)
			continue
		}
		applied := false
		for _, path := range paths {
			if !copyJSONPathValue(target, source, path) {
				result.Skipped = append(result.Skipped, raw+" -> "+path)
				continue
			}
			result.Applied = append(result.Applied, raw+" -> "+path)
			applied = true
		}
		if !applied {
			result.Skipped = append(result.Skipped, raw)
		}
	}
	return result
}

func normalizeMissingFileValue(raw string) string {
	return strings.TrimSpace(strings.TrimPrefix(raw, fileNotFoundPrefix))
}

func locateFileItemPaths(root map[string]any, validation gating.ValidationSpec, raw string) []string {
	paths := make([]string, 0, 2)
	for _, fileList := range validation.FileLists {
		value, ok := readJSONPath(root, fileList.ItemsPath)
		if !ok {
			continue
		}
		items, ok := value.([]any)
		if !ok {
			continue
		}
		for i, item := range items {
			current, ok := item.(string)
			if !ok || strings.TrimSpace(current) != raw {
				continue
			}
			paths = append(paths, fmt.Sprintf("%s[%d]", fileList.ItemsPath, i))
		}
	}
	return paths
}
