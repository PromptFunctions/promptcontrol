package scml

import (
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
)

type FileSystem interface {
	ReadFile(name string) ([]byte, error)
	Stat(name string) (iofs.FileInfo, error)
	Glob(pattern string) ([]string, error)
}

type ParseOptions struct {
	FS                  FileSystem
	SearchPaths         []string
	BaseDir             string
	SkipWriteValidation bool
}

type resolvedOptions struct {
	fs                  FileSystem
	searchPaths         []string
	baseDir             string
	skipWriteValidation bool
}

type osFS struct{}

func DefaultFS() FileSystem {
	return osFS{}
}

func (osFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (osFS) Stat(name string) (iofs.FileInfo, error) {
	return os.Stat(name)
}

func (osFS) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

func resolveParseOptions(opts *ParseOptions) resolvedOptions {
	if opts == nil {
		return resolvedOptions{fs: DefaultFS()}
	}

	var searchPaths []string
	if opts.SearchPaths != nil {
		searchPaths = copyStringSlice(opts.SearchPaths)
	}

	return resolvedOptions{
		fs:                  fsOrDefault(opts.FS),
		searchPaths:         searchPaths,
		baseDir:             cleanPath(opts.BaseDir),
		skipWriteValidation: opts.SkipWriteValidation,
	}
}

func (o resolvedOptions) searchRoots() []string {
	if o.searchPaths != nil {
		roots := make([]string, 0, len(o.searchPaths))
		for _, root := range o.searchPaths {
			cleaned := cleanPath(root)
			if cleaned == "" {
				continue
			}
			roots = append(roots, cleaned)
		}
		return roots
	}
	return defaultSCMLSearchRoots()
}

func fsOrDefault(fs FileSystem) FileSystem {
	if fs == nil {
		return DefaultFS()
	}
	return fs
}

func defaultSCMLSearchRoots() []string {
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
