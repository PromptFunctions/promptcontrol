package scml

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type dependencyLoader struct {
	cache   map[string]*Contract
	loading map[string]struct{}
	opts    resolvedOptions
}

func newDependencyLoader() *dependencyLoader {
	return newDependencyLoaderWithOptions(resolveParseOptions(nil))
}

func newDependencyLoaderWithOptions(opts resolvedOptions) *dependencyLoader {
	return &dependencyLoader{
		cache:   make(map[string]*Contract),
		loading: make(map[string]struct{}),
		opts:    opts,
	}
}

func (l *dependencyLoader) load(path string) (*Contract, error) {
	resolvedPath, err := resolveExistingFileFS(path, l.opts.fs)
	if err != nil {
		return nil, err
	}
	contract, err := l.loadResolved(resolvedPath)
	if err != nil {
		return nil, err
	}
	if !l.opts.skipWriteValidation {
		if _, err := ResolveContractWriteTargetsFS(contract, l.opts.fs); err != nil {
			return nil, err
		}
	}
	return contract, nil
}

func (l *dependencyLoader) loadFromReader(r io.Reader, baseDir string) (*Contract, error) {
	contract, err := parseContractFromReader(r, baseDir)
	if err != nil {
		return nil, err
	}
	if err := l.resolveContract(contract, baseDir); err != nil {
		return nil, err
	}
	if !l.opts.skipWriteValidation {
		if _, err := ResolveContractWriteTargetsFS(contract, l.opts.fs); err != nil {
			return nil, err
		}
	}
	return contract, nil
}

func (l *dependencyLoader) loadResolved(path string) (*Contract, error) {
	if cached, ok := l.cache[path]; ok {
		return cached, nil
	}
	if _, ok := l.loading[path]; ok {
		return nil, fmt.Errorf("import cycle detected at %q", path)
	}

	l.loading[path] = struct{}{}
	contract, err := parseLocalContractFile(path, l.opts.fs)
	if err != nil {
		delete(l.loading, path)
		return nil, err
	}

	if err := l.resolveContract(contract, filepath.Dir(path)); err != nil {
		delete(l.loading, path)
		return nil, err
	}

	l.cache[path] = contract
	delete(l.loading, path)
	return contract, nil
}

func (l *dependencyLoader) resolveContract(contract *Contract, _ string) error {
	if len(contract.Imports) == 0 {
		refreshContractViews(contract)
		return nil
	}

	importedModules := make(map[string]*Contract, len(contract.Imports))
	for _, entry := range contract.Imports {
		resolvedPath, err := resolveImportPathWithOptions(entry.Path, l.opts)
		if err != nil {
			return err
		}
		alias := resolveImportAlias(entry, resolvedPath)
		if alias == "" {
			return fmt.Errorf("import %q resolved to an empty alias", entry.Path)
		}
		if _, exists := importedModules[alias]; exists {
			return fmt.Errorf("duplicate import alias %q", alias)
		}
		module, err := l.loadResolved(resolvedPath)
		if err != nil {
			return err
		}
		importedModules[alias] = module
	}

	if err := bindSectionSources(contract.OrderedSections, importedModules); err != nil {
		return err
	}

	refreshContractViews(contract)
	return nil
}

func collectImportDeclarations(content string) ([]ImportEntry, string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var filtered strings.Builder
	imports := make([]ImportEntry, 0, 4)

	for lineNo := 1; scanner.Scan(); lineNo++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if isImportCommentCandidate(line) {
			start, err := parseImportMarker(line)
			if err != nil {
				return nil, "", fmt.Errorf("line %d: %w", lineNo, err)
			}
			path := requiredImportPathAttr(start)
			if path == "" {
				return nil, "", fmt.Errorf("line %d: <import> requires a path attribute", lineNo)
			}
			imports = append(imports, ImportEntry{
				Path: path,
				Name: optionalImportNameAttr(start),
			})
			filtered.WriteByte('\n')
			continue
		}
		filtered.WriteString(rawLine)
		filtered.WriteByte('\n')
	}

	if err := scanner.Err(); err != nil {
		return nil, "", fmt.Errorf("scan SCML imports: %w", err)
	}

	return imports, filtered.String(), nil
}

func bindSectionSources(sections []SectionEntry, modules map[string]*Contract) error {
	for i := range sections {
		if err := bindSectionSource(&sections[i], modules); err != nil {
			return err
		}
	}
	return nil
}

func bindSectionSource(section *SectionEntry, modules map[string]*Contract) error {
	if section.DataSource != "" {
		return bindSectionDataSource(section, modules)
	}

	for i := range section.Routes {
		if err := bindRouteSource(&section.Routes[i], modules); err != nil {
			return err
		}
	}
	return nil
}

func bindRouteSource(route *RouteNode, modules map[string]*Contract) error {
	if route.DataSource != "" {
		return bindRouteDataSource(route, modules)
	}

	for i := range route.Children {
		if err := bindRouteSource(&route.Children[i], modules); err != nil {
			return err
		}
	}
	return nil
}

func bindSectionDataSource(section *SectionEntry, modules map[string]*Contract) error {
	alias, targetSection, err := splitDataSourceRef(section.DataSource)
	if err != nil {
		return fmt.Errorf("section %q: %w", section.Name, err)
	}
	module, ok := modules[alias]
	if !ok {
		return fmt.Errorf("section %q imports unknown module alias %q", section.Name, alias)
	}

	imported, ok := lookupSectionEntry(module.OrderedSections, targetSection)
	if !ok {
		return fmt.Errorf("section %q imports unknown section %q in module %q", section.Name, targetSection, alias)
	}
	if imported.Name != section.Name {
		return fmt.Errorf("section %q imports mismatched section %q from module %q", section.Name, imported.Name, alias)
	}

	section.DataType = imported.DataType
	section.DataPolicy = imported.DataPolicy
	section.Items = copyStringSlice(imported.Items)
	section.Routes = copyRouteNodes(imported.Routes)
	section.sourceDir = imported.sourceDir
	return nil
}

func bindRouteDataSource(route *RouteNode, modules map[string]*Contract) error {
	alias, targetSection, err := splitDataSourceRef(route.DataSource)
	if err != nil {
		return fmt.Errorf("section %q: %w", route.Term, err)
	}
	module, ok := modules[alias]
	if !ok {
		return fmt.Errorf("section %q imports unknown module alias %q", route.Term, alias)
	}

	imported, ok := lookupSectionEntry(module.OrderedSections, targetSection)
	if !ok {
		return fmt.Errorf("section %q imports unknown section %q in module %q", route.Term, targetSection, alias)
	}
	if imported.Name != route.Term {
		return fmt.Errorf("section %q imports mismatched section %q from module %q", route.Term, imported.Name, alias)
	}

	route.DataType = imported.DataType
	route.DataPolicy = imported.DataPolicy
	route.Items = copyStringSlice(imported.Items)
	route.Children = copyRouteNodes(imported.Routes)
	route.sourceDir = imported.sourceDir
	return nil
}

func lookupSectionEntry(sections []SectionEntry, name string) (SectionEntry, bool) {
	for _, section := range sections {
		if section.Name == name {
			return copySectionEntry(section), true
		}
	}
	return SectionEntry{}, false
}

func splitDataSourceRef(ref string) (string, string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", "", fmt.Errorf("data-source cannot be empty")
	}

	alias, section, ok := strings.Cut(trimmed, ".")
	if !ok {
		return "", "", fmt.Errorf("data-source %q must use alias.section format", trimmed)
	}
	alias = strings.TrimSpace(alias)
	section = strings.TrimSpace(section)
	if alias == "" || section == "" {
		return "", "", fmt.Errorf("data-source %q must use alias.section format", trimmed)
	}
	return alias, section, nil
}

func requiredImportPathAttr(start xml.StartElement) string {
	value, ok, err := xmlAttrValue(start, "path")
	if err != nil || !ok {
		return ""
	}
	return value
}

func optionalImportNameAttr(start xml.StartElement) string {
	value, ok, err := xmlAttrValue(start, "name")
	if err != nil || !ok {
		return ""
	}
	return value
}

func parseImportMarker(line string) (xml.StartElement, error) {
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "<!--"), "-->"))
	inner = strings.TrimSpace(inner)
	if !strings.HasPrefix(inner, "<import") || !strings.HasSuffix(inner, "/>") {
		return xml.StartElement{}, fmt.Errorf("invalid import marker %q", line)
	}

	decoder := xml.NewDecoder(strings.NewReader(inner))
	tok, err := decoder.Token()
	if err != nil {
		return xml.StartElement{}, fmt.Errorf("parse import marker: %w", err)
	}
	start, ok := tok.(xml.StartElement)
	if !ok {
		return xml.StartElement{}, fmt.Errorf("parse import marker: expected start element")
	}
	if start.Name.Local != "import" {
		return xml.StartElement{}, fmt.Errorf("invalid import marker %q", line)
	}
	return start, nil
}

func isImportCommentCandidate(line string) bool {
	return strings.HasPrefix(line, "<!-- <import ") && strings.HasSuffix(line, " -->")
}

func refreshContractViews(contract *Contract) {
	sectionsMap := make(map[string][]string, len(contract.OrderedSections))
	for _, section := range contract.OrderedSections {
		sectionsMap[section.Name] = copyStringSlice(section.Items)
	}
	contract.Sections = sectionsMap
	contract.SectionRoutes = buildSectionRoutes(contract.OrderedSections)
}

func ResolveContractWriteTargets(contract *Contract) (WriteTargetResolution, error) {
	return ResolveContractWriteTargetsFS(contract, DefaultFS())
}

func ResolveContractWriteTargetsFS(contract *Contract, fs FileSystem) (WriteTargetResolution, error) {
	targets := collectWritableTargets(contract.OrderedSections)
	if len(targets) == 0 {
		return WriteTargetResolution{}, nil
	}
	return ResolveWriteTargetsFS(targets, fs)
}

func collectWritableTargets(sections []SectionEntry) []WriteTargetRequest {
	targets := make([]WriteTargetRequest, 0, 16)
	for _, section := range sections {
		targets = appendWritableSectionTargets(targets, section)
	}
	return targets
}

func appendWritableSectionTargets(targets []WriteTargetRequest, section SectionEntry) []WriteTargetRequest {
	if section.DataType == "file-list" && section.DataPolicy == "write" {
		for _, item := range section.Items {
			targets = append(targets, WriteTargetRequest{Path: item, BaseDir: section.sourceDir})
		}
	}
	for _, route := range section.Routes {
		targets = appendWritableRouteTargets(targets, route)
	}
	return targets
}

func appendWritableRouteTargets(targets []WriteTargetRequest, route RouteNode) []WriteTargetRequest {
	if route.DataType == "file-list" && route.DataPolicy == "write" {
		for _, item := range route.Items {
			targets = append(targets, WriteTargetRequest{Path: item, BaseDir: route.sourceDir})
		}
	}
	for _, child := range route.Children {
		targets = appendWritableRouteTargets(targets, child)
	}
	return targets
}
