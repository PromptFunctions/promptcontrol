package scml

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type WriteTargetRequest struct {
	Path    string
	BaseDir string
}

type WriteTargetResolution struct {
	Targets []WriteTargetRequest
	Found   map[string][]string
	Missing []string
}

type MissingWriteTargetsError struct {
	Missing []string
}

func (e *MissingWriteTargetsError) Error() string {
	return "no targeted files were found on disk"
}

func resolveImportPath(spec string) (string, error) {
	return resolveImportPathWithOptions(spec, resolveParseOptions(nil))
}

func resolveImportPathWithOptions(spec string, opts resolvedOptions) (string, error) {
	pathSpec, err := validateImportPathSpec(spec)
	if err != nil {
		return "", err
	}
	return resolveImportSearchPathFS(pathSpec, opts)
}

func resolveImportAlias(entry ImportEntry, resolvedPath string) string {
	name := strings.TrimSpace(entry.Name)
	if name != "" {
		return name
	}
	stem := strings.TrimSuffix(filepath.Base(resolvedPath), filepath.Ext(resolvedPath))
	if stem == "" {
		return "module"
	}
	return strings.ToLower(stem)
}

func validateImportPathSpec(spec string) (string, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return "", fmt.Errorf("import path cannot be empty")
	}

	switch strings.ToLower(filepath.Ext(trimmed)) {
	case ".md", ".scml":
		return trimmed, nil
	default:
		return "", fmt.Errorf("unsupported import file extension %q", filepath.Ext(trimmed))
	}
}

func resolveImportSearchPath(pathSpec string) (string, error) {
	return resolveImportSearchPathFS(pathSpec, resolveParseOptions(nil))
}

func resolveImportSearchPathFS(pathSpec string, opts resolvedOptions) (string, error) {
	if filepath.IsAbs(pathSpec) {
		return resolveExistingFileFS(pathSpec, opts.fs)
	}

	roots := opts.searchRoots()
	for _, root := range roots {
		candidate := cleanPath(filepath.Join(root, pathSpec))
		if fileExistsFS(candidate, opts.fs) {
			return resolveExistingFileFS(candidate, opts.fs)
		}
	}

	if opts.searchPaths != nil {
		return "", fmt.Errorf("unable to resolve SCML import %q from configured search paths", pathSpec)
	}
	return "", fmt.Errorf("unable to resolve SCML import %q from SCMLPATH", pathSpec)
}

func resolveExistingFile(path string) (string, error) {
	return resolveExistingFileFS(path, DefaultFS())
}

func resolveExistingFileFS(path string, fs FileSystem) (string, error) {
	resolved, err := filepath.Abs(cleanPath(path))
	if err != nil {
		return "", err
	}
	if !fileExistsFS(resolved, fs) {
		return "", fmt.Errorf("file does not exist: %s", resolved)
	}
	return resolved, nil
}

func ResolveWriteTargets(targets []WriteTargetRequest) (WriteTargetResolution, error) {
	return ResolveWriteTargetsFS(targets, DefaultFS())
}

func ResolveWriteTargetsFS(targets []WriteTargetRequest, fs FileSystem) (WriteTargetResolution, error) {
	resolution := WriteTargetResolution{
		Targets: append([]WriteTargetRequest(nil), targets...),
		Found:   make(map[string][]string, len(targets)),
		Missing: make([]string, 0, len(targets)),
	}
	seenMissing := make(map[string]struct{}, len(targets))
	totalFound := 0

	for _, target := range targets {
		raw := strings.TrimSpace(target.Path)
		if raw == "" {
			continue
		}

		baseDir := strings.TrimSpace(target.BaseDir)
		matches, err := resolveWriteTargetFS(baseDir, raw, fs)
		if err != nil {
			return WriteTargetResolution{}, err
		}
		if len(matches) == 0 {
			if _, ok := seenMissing[raw]; !ok {
				seenMissing[raw] = struct{}{}
				resolution.Missing = append(resolution.Missing, raw)
			}
			continue
		}

		resolution.Found[raw] = matches
		totalFound += len(matches)
	}

	if totalFound == 0 {
		return resolution, &MissingWriteTargetsError{
			Missing: append([]string(nil), resolution.Missing...),
		}
	}

	return resolution, nil
}

func ResolveWriteTargetMatches(baseDir, raw string) ([]string, error) {
	return ResolveWriteTargetMatchesFS(baseDir, raw, DefaultFS())
}

func ResolveWriteTargetMatchesFS(baseDir, raw string, fs FileSystem) ([]string, error) {
	return resolveWriteTargetFS(strings.TrimSpace(baseDir), strings.TrimSpace(raw), fs)
}

func resolveWriteTargetFS(baseDir, raw string, fs FileSystem) ([]string, error) {
	if raw == "" {
		return nil, nil
	}

	candidate := raw
	if !filepath.IsAbs(raw) {
		candidate = cleanPath(filepath.Join(baseDir, raw))
	}

	if HasGlobMeta(raw) {
		matches, err := fs.Glob(candidate)
		if err != nil {
			return nil, fmt.Errorf("invalid write glob %q: %w", raw, err)
		}
		return FilterFileMatches(matches, fs), nil
	}

	info, err := fs.Stat(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, nil
	}

	resolved, err := filepath.Abs(candidate)
	if err != nil {
		return nil, err
	}
	return []string{resolved}, nil
}

func FilterFileMatches(matches []string, fs FileSystem) []string {
	filtered := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		info, err := fs.Stat(match)
		if err != nil || info.IsDir() {
			continue
		}
		resolved, err := filepath.Abs(match)
		if err != nil {
			continue
		}
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		filtered = append(filtered, resolved)
	}
	return filtered
}

func scmlSearchRoots() []string {
	raw := strings.TrimSpace(os.Getenv("SCMLPATH"))
	if raw == "" {
		return []string{"."}
	}

	parts := strings.Split(raw, string(os.PathListSeparator))
	roots := make([]string, 0, len(parts))
	for _, part := range parts {
		root := strings.TrimSpace(part)
		if root == "" {
			continue
		}
		roots = append(roots, root)
	}
	if len(roots) == 0 {
		return []string{"."}
	}
	return roots
}

func fileExists(path string) bool {
	return fileExistsFS(path, DefaultFS())
}

func fileExistsFS(path string, fs FileSystem) bool {
	info, err := fs.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func HasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}
