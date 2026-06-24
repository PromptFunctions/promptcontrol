package forge

import (
	"encoding/json"
	"strconv"
	"strings"
)

type pathStep struct {
	Key   string
	Index int
	IsKey bool
}

func marshalIndentedJSON(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "    ")
}

func copyJSONPathValue(target, source map[string]any, path string) bool {
	value, ok := readJSONPath(source, path)
	if !ok {
		return false
	}
	_, ok = writeJSONPath(target, path, cloneJSONValue(value))
	return ok
}

func readJSONPath(root map[string]any, path string) (any, bool) {
	steps, ok := parseJSONPath(path)
	if !ok {
		return nil, false
	}

	current := any(root)
	for _, step := range steps {
		if step.IsKey {
			object, ok := current.(map[string]any)
			if !ok {
				return nil, false
			}
			next, ok := object[step.Key]
			if !ok {
				return nil, false
			}
			current = next
			continue
		}

		items, ok := current.([]any)
		if !ok || step.Index < 0 || step.Index >= len(items) {
			return nil, false
		}
		current = items[step.Index]
	}
	return current, true
}

func writeJSONPath(root map[string]any, path string, value any) (map[string]any, bool) {
	steps, ok := parseJSONPath(path)
	if !ok {
		return nil, false
	}

	updated, ok := writePathValue(root, steps, value)
	if !ok {
		return nil, false
	}
	result, ok := updated.(map[string]any)
	if !ok {
		return nil, false
	}
	return result, true
}

func writePathValue(current any, steps []pathStep, value any) (any, bool) {
	if len(steps) == 0 {
		return value, true
	}

	step := steps[0]
	if step.IsKey {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		child, exists := object[step.Key]
		if !exists || child == nil {
			child = newContainerForStep(steps[1:])
		}
		updatedChild, ok := writePathValue(child, steps[1:], value)
		if !ok {
			return nil, false
		}
		object[step.Key] = updatedChild
		return object, true
	}

	items, ok := current.([]any)
	if !ok {
		return nil, false
	}
	for len(items) <= step.Index {
		items = append(items, nil)
	}
	if len(steps) == 1 {
		items[step.Index] = value
		return items, true
	}

	child := items[step.Index]
	if child == nil {
		child = newContainerForStep(steps[1:])
	}
	updatedChild, ok := writePathValue(child, steps[1:], value)
	if !ok {
		return nil, false
	}
	items[step.Index] = updatedChild
	return items, true
}

func newContainerForStep(steps []pathStep) any {
	if len(steps) == 0 {
		return nil
	}
	if steps[0].IsKey {
		return make(map[string]any)
	}
	return make([]any, 0, steps[0].Index+1)
}

func parseJSONPath(path string) ([]pathStep, bool) {
	if path == "" {
		return nil, false
	}

	segments := strings.Split(path, ".")
	steps := make([]pathStep, 0, len(segments)*2)
	for _, segment := range segments {
		name, indexes, ok := parseForgePathSegment(segment)
		if !ok {
			return nil, false
		}
		if name != "" {
			steps = append(steps, pathStep{Key: name, IsKey: true})
		}
		for _, index := range indexes {
			steps = append(steps, pathStep{Index: index})
		}
	}
	if len(steps) == 0 {
		return nil, false
	}
	return steps, true
}

func isStructuredMissingPath(path string) bool {
	if !(path == "Title" || strings.HasPrefix(path, "Constants[") || strings.HasPrefix(path, "Sections[")) {
		return false
	}
	_, ok := parseJSONPath(path)
	return ok
}

func parseForgePathSegment(segment string) (string, []int, bool) {
	if segment == "" {
		return "", nil, false
	}
	name := segment
	indexes := make([]int, 0, 2)
	if bracket := strings.IndexByte(segment, '['); bracket >= 0 {
		name = segment[:bracket]
		rest := segment[bracket:]
		for len(rest) > 0 {
			if rest[0] != '[' {
				return "", nil, false
			}
			end := strings.IndexByte(rest, ']')
			if end <= 1 {
				return "", nil, false
			}
			value, err := strconv.Atoi(rest[1:end])
			if err != nil {
				return "", nil, false
			}
			indexes = append(indexes, value)
			rest = rest[end+1:]
		}
	}
	return name, indexes, true
}

func cloneJSONMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneJSONValue(value)
	}
	return out
}

func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneJSONMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = cloneJSONValue(typed[i])
		}
		return out
	default:
		return typed
	}
}
