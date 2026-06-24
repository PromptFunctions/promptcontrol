package forge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/PromptFunctions/promptcontrol/dev-contracts/gating"
	"github.com/PromptFunctions/promptcontrol/dev-contracts/scml"
)

const (
	debugListLimit    = 8
	treeSnapshotLimit = 64
)

type missingDisplayGroup struct {
	Title string
	Items []string
}

func buildForgePrompt(referenceShape, checkpoint string, missing []string) string {
	var builder strings.Builder
	builder.WriteString("Fill the rendered SCML contract JSON.\n")
	builder.WriteString("The reference shape is authoritative.\n")
	builder.WriteString("The checkpoint is the current contract state: preserve populated values and fill blank values only.\n")
	builder.WriteString("Any string wrapped as [LLM TO FILL INSTRUCTIONS: ...] is a placeholder and must be replaced, not copied verbatim.\n")
	if len(missing) > 0 {
		builder.WriteString("- Preserve already-correct values and fill only the listed missing fields/items.\n")
		builder.WriteString("- Do not rewrite unrelated sections or routes.\n")
		builder.WriteString("Previous attempt failed validation. Fix these missing fields or files:\n")
		for _, item := range missing {
			builder.WriteString("- ")
			builder.WriteString(item)
			builder.WriteByte('\n')
		}
	}
	builder.WriteString("Reference shape:\n")
	builder.WriteString(referenceShape)
	builder.WriteString("\nCurrent checkpoint:\n")
	builder.WriteString(checkpoint)
	return builder.String()
}

func buildForgeSystemPrompt(validation gating.ValidationSpec) (string, error) {
	treeSnapshot, err := buildMockTreeSnapshot(validation)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("This is a mock exercise.\n")
	builder.WriteString("Fill the JSON form by reading the instruction at each line and replacing it with a speculative action in a coherent mock coding plan.\n")
	builder.WriteString("Do not copy instruction lines back verbatim.\n")
	builder.WriteString("Keep the existing JSON structure and metadata fields exactly as provided.\n")
	builder.WriteString("Use concise, concrete mock values that read like an actual coding plan.\n")
	builder.WriteString("For every file-list items array, choose only file paths that exist in the provided on-disk tree snapshot.\n")
	builder.WriteString("If a file path is not present in the tree snapshot, do not return it.\n")
	builder.WriteString("Mock tree snapshot:\n")
	builder.WriteString(treeSnapshot)
	return builder.String(), nil
}

func writeFormattedMissing(w io.Writer, reference scml.RenderContract, missing []string) {
	if w == nil {
		return
	}
	for _, group := range formatMissingGroups(reference, missing) {
		fmt.Fprintf(w, "%s:\n", group.Title)
		for _, item := range group.Items {
			fmt.Fprintf(w, "  - %s\n", item)
		}
	}
}

func formatMissingGroups(reference scml.RenderContract, missing []string) []missingDisplayGroup {
	groups := []missingDisplayGroup{
		{Title: "missing section items"},
		{Title: "missing route items"},
		{Title: "missing fields"},
		{Title: "missing files"},
		{Title: "other missing values"},
	}
	seen := make([]map[string]struct{}, len(groups))
	for i := range seen {
		seen[i] = make(map[string]struct{})
	}

	for _, raw := range missing {
		groupIdx, label := resolveMissingLabel(reference, raw)
		if _, ok := seen[groupIdx][label]; ok {
			continue
		}
		seen[groupIdx][label] = struct{}{}
		groups[groupIdx].Items = append(groups[groupIdx].Items, label)
	}

	filtered := make([]missingDisplayGroup, 0, len(groups))
	for _, group := range groups {
		if len(group.Items) == 0 {
			continue
		}
		filtered = append(filtered, group)
	}
	return filtered
}

func resolveMissingLabel(reference scml.RenderContract, raw string) (int, string) {
	if strings.HasPrefix(raw, fileNotFoundPrefix) {
		return 3, raw
	}
	sectionIdx, routeIdx, field, ok := parseMissingPath(raw)
	if !ok {
		if looksLikeFileValue(raw) {
			return 3, raw
		}
		return 4, raw
	}
	if sectionIdx < 0 || sectionIdx >= len(reference.Sections) {
		return 4, raw
	}

	section := reference.Sections[sectionIdx]
	if routeIdx == nil {
		if field == "Items" {
			return 0, section.Name
		}
		return 2, fmt.Sprintf("%s missing %s", section.Name, displayMissingField(field))
	}

	if *routeIdx < 0 || *routeIdx >= len(section.Routes) {
		return 4, raw
	}
	route := section.Routes[*routeIdx]
	if field == "" {
		return 2, fmt.Sprintf("%s missing route entry", route.Path)
	}
	if field == "Items" {
		return 1, route.Path
	}
	return 2, fmt.Sprintf("%s missing %s", route.Path, displayMissingField(field))
}

func parseMissingPath(raw string) (int, *int, string, bool) {
	if !strings.HasPrefix(raw, "Sections[") {
		return 0, nil, "", false
	}

	sectionIdx, rest, ok := consumeIndex(strings.TrimPrefix(raw, "Sections["))
	if !ok || rest == "" {
		return 0, nil, "", false
	}
	rest = strings.TrimPrefix(rest, ".")
	if strings.HasPrefix(rest, "Items") {
		return sectionIdx, nil, "Items", true
	}
	if !strings.HasPrefix(rest, "Routes[") {
		return sectionIdx, nil, strings.TrimPrefix(rest, "."), true
	}

	routeIdx, rest, ok := consumeIndex(strings.TrimPrefix(rest, "Routes["))
	if !ok {
		return 0, nil, "", false
	}
	if rest == "" {
		return sectionIdx, &routeIdx, "", true
	}
	rest = strings.TrimPrefix(rest, ".")
	field := rest
	if bracket := strings.IndexByte(field, '['); bracket >= 0 {
		field = field[:bracket]
	}
	return sectionIdx, &routeIdx, field, true
}

func consumeIndex(raw string) (int, string, bool) {
	end := strings.IndexByte(raw, ']')
	if end <= 0 {
		return 0, "", false
	}
	value, err := strconv.Atoi(raw[:end])
	if err != nil {
		return 0, "", false
	}
	return value, raw[end+1:], true
}

func displayMissingField(field string) string {
	if field == "" {
		return "field"
	}

	var builder strings.Builder
	for i, r := range field {
		if i > 0 && r >= 'A' && r <= 'Z' {
			builder.WriteByte('-')
		}
		builder.WriteRune(r)
	}
	return strings.ToLower(builder.String())
}

func looksLikeFileValue(raw string) bool {
	if strings.ContainsAny(raw, " \t\n") {
		return false
	}
	return strings.Contains(raw, "/") || strings.Contains(raw, ".") || strings.Contains(raw, "*")
}

func writeTraceBlock(w io.Writer, title, content string) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "%s:\n", title)
	fmt.Fprintln(w, content)
}

func writeReturnedKeys(w io.Writer, candidate map[string]any, expected []string) {
	if w == nil {
		return
	}
	keys := flattenReturnedKeys(candidate)
	matched := countMatchingKeys(keys, expected)
	missing := len(expected) - matched
	fmt.Fprintf(w, "returned keys count: %d%% (%d/%d)\n", coveragePercent(matched, len(expected)), matched, len(expected))
	fmt.Fprintf(w, "missing keys count: %d%% (%d/%d)\n", coveragePercent(missing, len(expected)), missing, len(expected))
	fmt.Fprintln(w, "returned keys:")
	for _, key := range keys {
		fmt.Fprintf(w, "  - %s\n", key)
	}
}

func expectedReturnedKeys(reference scml.RenderContract, validation gating.ValidationSpec) []string {
	keys := flattenReturnedKeys(referenceKeyRoot(reference))
	filtered := make([]string, 0, len(keys))
	for _, key := range keys {
		if shouldSkipExpectedKey(key, validation) {
			continue
		}
		filtered = append(filtered, key)
	}
	return filtered
}

func referenceKeyRoot(reference scml.RenderContract) map[string]any {
	data, err := json.Marshal(reference)
	if err != nil {
		return buildReferenceShape(reference)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return buildReferenceShape(reference)
	}
	return root
}

func flattenReturnedKeys(candidate map[string]any) []string {
	keys := make([]string, 0, 32)
	collectReturnedKeys(&keys, "", candidate)
	sort.Strings(keys)
	return keys
}

func shouldSkipExpectedKey(key string, validation gating.ValidationSpec) bool {
	for _, fileList := range validation.FileLists {
		if fileList.Mode != "open-cardinality" {
			continue
		}
		if strings.HasPrefix(key, fileList.ItemsPath+"[") {
			return true
		}
	}
	return false
}

func countMatchingKeys(returned, expected []string) int {
	if len(returned) == 0 || len(expected) == 0 {
		return 0
	}
	returnedSet := make(map[string]struct{}, len(returned))
	for _, key := range returned {
		returnedSet[key] = struct{}{}
	}
	matched := 0
	for _, key := range expected {
		if _, ok := returnedSet[key]; ok {
			matched++
		}
	}
	return matched
}

func coveragePercent(count, total int) int {
	if total <= 0 {
		return 100
	}
	if count <= 0 {
		return 0
	}
	return (count*100 + total/2) / total
}

func collectReturnedKeys(keys *[]string, prefix string, value any) {
	switch typed := value.(type) {
	case map[string]any:
		objectKeys := make([]string, 0, len(typed))
		for key := range typed {
			objectKeys = append(objectKeys, key)
		}
		sort.Strings(objectKeys)
		for _, key := range objectKeys {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			*keys = append(*keys, path)
			collectReturnedKeys(keys, path, typed[key])
		}
	case []any:
		for i, item := range typed {
			path := fmt.Sprintf("%s[%d]", prefix, i)
			*keys = append(*keys, path)
			collectReturnedKeys(keys, path, item)
		}
	}
}

func writeAttemptStart(w io.Writer, attempt, maxRetries int, missing []string, promptBytes int) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "attempt %d/%d\n", attempt, maxRetries)
	if len(missing) == 0 {
		fmt.Fprintln(w, "mode: initial scaffold")
	} else {
		fmt.Fprintln(w, "mode: fill missing pieces")
		writeBoundedList(w, "requested fixes", missing)
	}
	fmt.Fprintf(w, "prompt bytes: %d\n", promptBytes)
}

func writeMergeResult(w io.Writer, merge mergeResult) {
	if w == nil {
		return
	}
	writeBoundedList(w, "applied updates", merge.Applied)
	if len(merge.Skipped) > 0 {
		writeBoundedList(w, "not returned by retry payload", merge.Skipped)
	}
}

func writeBoundedList(w io.Writer, title string, values []string) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "%s: %d\n", title, len(values))
	limit := len(values)
	if limit > debugListLimit {
		limit = debugListLimit
	}
	for i := 0; i < limit; i++ {
		fmt.Fprintf(w, "  - %s\n", values[i])
	}
	if len(values) > limit {
		fmt.Fprintf(w, "  - ... and %d more\n", len(values)-limit)
	}
}

func buildMockTreeSnapshot(validation gating.ValidationSpec) (string, error) {
	baseDirs := distinctValidationBaseDirs(validation)
	if len(baseDirs) == 0 {
		return "(no file-list directories)\n", nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	for _, baseDir := range baseDirs {
		label := displayBaseDir(cwd, baseDir)
		builder.WriteString("root: ")
		builder.WriteString(label)
		builder.WriteByte('\n')

		entries, truncated, err := listTreeEntries(baseDir, treeSnapshotLimit)
		if err != nil {
			return "", err
		}
		if len(entries) == 0 {
			builder.WriteString("  - (no files)\n")
		} else {
			for _, entry := range entries {
				builder.WriteString("  - ")
				builder.WriteString(entry)
				builder.WriteByte('\n')
			}
		}
		if truncated {
			builder.WriteString("  - ... truncated\n")
		}
	}
	return builder.String(), nil
}

func distinctValidationBaseDirs(validation gating.ValidationSpec) []string {
	seen := make(map[string]struct{}, len(validation.FileLists))
	baseDirs := make([]string, 0, len(validation.FileLists))
	for _, fileList := range validation.FileLists {
		baseDir := strings.TrimSpace(fileList.BaseDir)
		if baseDir == "" {
			continue
		}
		if _, ok := seen[baseDir]; ok {
			continue
		}
		seen[baseDir] = struct{}{}
		baseDirs = append(baseDirs, baseDir)
	}
	sort.Strings(baseDirs)
	return baseDirs
}

func displayBaseDir(cwd, baseDir string) string {
	rel, err := filepath.Rel(cwd, baseDir)
	if err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
		return filepath.Clean(rel)
	}
	return filepath.Base(baseDir)
}

func listTreeEntries(baseDir string, limit int) ([]string, bool, error) {
	entries := make([]string, 0, limit)
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		entries = append(entries, filepath.Clean(rel))
		if len(entries) > limit {
			return io.EOF
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	sort.Strings(entries)
	if len(entries) > limit {
		return entries[:limit], true, nil
	}
	return entries, errors.Is(err, io.EOF), nil
}
