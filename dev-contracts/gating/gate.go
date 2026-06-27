/*
Prompt Control
==============

What this file is:
- A stateless validation engine for rendered JSON contracts.
- The validation half of the forge workflow used by benchmark/main.go.

What it validates:
1) Recursive structural presence of rendered sections, routes, and fields
2) Exact value preservation for scaffold identity fields
3) Non-empty item population for expected item slots
4) File-list disk validation through ValidationSpec

Return contract:
- ValidationResult{Status, Missing}
- Status is "completed" when validation succeeds
- Status is "missing_fields" with deterministic sorted paths otherwise
*/
package gating

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	scml "github.com/PromptFunctions/promptcontrol/dev-contracts/contracts"
)

const (
	JSONContractCompletedStatus = "completed"
	JSONContractMissingStatus   = "missing_fields"
)

func parseJSONFieldMeta(tag, fallback string) (string, bool) {
	if tag == "" {
		return lowerFirst(fallback), false
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = lowerFirst(fallback)
	}
	for _, part := range parts[1:] {
		if part == "omitempty" {
			return name, true
		}
	}
	return name, false
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	if len(s) == 1 {
		return strings.ToLower(s)
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func dedupeSorted(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := values[:1]
	for i := 1; i < len(values); i++ {
		if values[i] == values[i-1] {
			continue
		}
		out = append(out, values[i])
	}
	return out
}

type FileListSpec struct {
	ItemsPath string
	BaseDir   string
	Mode      string
}

type ValidationSpec struct {
	FileLists []FileListSpec
}

type ValidationResult struct {
	Status  string
	Missing []string
}

func ValidateContract(candidate map[string]any, reference any, spec ValidationSpec) ValidationResult {
	missing := make([]string, 0, 32)
	fileLists := indexFileLists(spec)
	validateRenderedValue(reflect.ValueOf(reference), candidate, "", fileLists, &missing)
	missing = append(missing, collectMissingRenderFiles(candidate, spec)...)
	sort.Strings(missing)
	missing = dedupeSorted(missing)
	if len(missing) == 0 {
		return ValidationResult{Status: JSONContractCompletedStatus}
	}
	return ValidationResult{
		Status:  JSONContractMissingStatus,
		Missing: missing,
	}
}

func indexFileLists(spec ValidationSpec) map[string]FileListSpec {
	indexed := make(map[string]FileListSpec, len(spec.FileLists))
	for _, fileList := range spec.FileLists {
		indexed[fileList.ItemsPath] = fileList
	}
	return indexed
}

func validateRenderedValue(reference reflect.Value, candidate any, path string, fileLists map[string]FileListSpec, missing *[]string) {
	if !reference.IsValid() {
		return
	}
	for reference.Kind() == reflect.Pointer {
		if reference.IsNil() {
			return
		}
		reference = reference.Elem()
	}

	switch reference.Kind() {
	case reflect.Struct:
		candidateMap, ok := candidate.(map[string]any)
		if !ok {
			if path != "" {
				*missing = append(*missing, path)
			}
			return
		}
		referenceType := reference.Type()
		for i := 0; i < reference.NumField(); i++ {
			field := referenceType.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name, omitempty := parseJSONFieldMeta(field.Tag.Get("json"), field.Name)
			if name == "-" || name == "" {
				continue
			}
			if omitempty && isOmittableZero(reference.Field(i)) {
				continue
			}
			childPath := joinPath(path, name)
			childCandidate, ok := candidateMap[name]
			if !ok {
				*missing = append(*missing, childPath)
				continue
			}
			if field.Name == "Items" && reference.Field(i).Kind() == reflect.Slice {
				candidateItems, ok := childCandidate.([]any)
				if !ok {
					*missing = append(*missing, childPath)
					continue
				}
				if fileList, ok := fileLists[childPath]; ok && fileList.Mode == "open-cardinality" {
					for _, item := range candidateItems {
						value, ok := item.(string)
						if !ok || strings.TrimSpace(value) == "" {
							*missing = append(*missing, childPath)
						}
					}
					continue
				}
				referenceItems := reference.Field(i)
				for idx := 0; idx < referenceItems.Len(); idx++ {
					itemPath := fmt.Sprintf("%s[%d]", childPath, idx)
					if idx >= len(candidateItems) {
						*missing = append(*missing, itemPath)
						continue
					}
					value, ok := candidateItems[idx].(string)
					if !ok || strings.TrimSpace(value) == "" {
						*missing = append(*missing, itemPath)
					}
				}
				continue
			}
			validateRenderedValue(reference.Field(i), childCandidate, childPath, fileLists, missing)
		}
	case reflect.Slice, reflect.Array:
		candidateSlice, ok := candidate.([]any)
		if !ok {
			*missing = append(*missing, path)
			return
		}
		for i := 0; i < reference.Len(); i++ {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			if i >= len(candidateSlice) {
				*missing = append(*missing, childPath)
				continue
			}
			validateRenderedValue(reference.Index(i), candidateSlice[i], childPath, fileLists, missing)
		}
	case reflect.String:
		value, ok := candidate.(string)
		if !ok || value != reference.String() {
			*missing = append(*missing, path)
		}
	case reflect.Bool:
		value, ok := candidate.(bool)
		if !ok || value != reference.Bool() {
			*missing = append(*missing, path)
		}
	default:
		if !reflect.DeepEqual(candidate, reference.Interface()) {
			*missing = append(*missing, path)
		}
	}
}

func collectMissingRenderFiles(candidate map[string]any, spec ValidationSpec) []string {
	missing := make([]string, 0, len(spec.FileLists))
	for _, fileList := range spec.FileLists {
		value, ok := lookupJSONPath(candidate, fileList.ItemsPath)
		if !ok {
			missing = append(missing, fileList.ItemsPath)
			continue
		}
		items, ok := value.([]any)
		if !ok || len(items) == 0 {
			missing = append(missing, fileList.ItemsPath)
			continue
		}

		for _, item := range items {
			raw, ok := item.(string)
			if !ok || strings.TrimSpace(raw) == "" {
				missing = append(missing, fileList.ItemsPath)
				continue
			}
			matches, err := resolveValidationTargetMatches(fileList.BaseDir, raw)
			if err != nil || len(matches) == 0 {
				missing = append(missing, "file not found on disk: "+filepath.Clean(strings.TrimSpace(raw)))
			}
		}
	}
	return missing
}

func lookupJSONPath(root map[string]any, path string) (any, bool) {
	current := any(root)
	for _, segment := range strings.Split(path, ".") {
		name, indexes, ok := parsePathSegment(segment)
		if !ok {
			return nil, false
		}
		if name != "" {
			object, ok := current.(map[string]any)
			if !ok {
				return nil, false
			}
			next, ok := object[name]
			if !ok {
				return nil, false
			}
			current = next
		}
		for _, index := range indexes {
			items, ok := current.([]any)
			if !ok || index < 0 || index >= len(items) {
				return nil, false
			}
			current = items[index]
		}
	}
	return current, true
}

func parsePathSegment(segment string) (string, []int, bool) {
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
			value := 0
			for _, r := range rest[1:end] {
				if r < '0' || r > '9' {
					return "", nil, false
				}
				value = value*10 + int(r-'0')
			}
			indexes = append(indexes, value)
			rest = rest[end+1:]
		}
	}
	return name, indexes, true
}

func isOmittableZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map:
		return v.IsNil() || (v.Kind() == reflect.Map && v.Len() == 0)
	case reflect.Slice, reflect.Array, reflect.String:
		return v.Len() == 0
	default:
		return v.IsZero()
	}
}

func resolveValidationTargetMatches(baseDir, raw string) ([]string, error) {
	return scml.ResolveWriteTargetMatchesFS(baseDir, raw, scml.DefaultFS())
}
