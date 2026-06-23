package scml

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var sectionDataTypeValues = map[string]struct{}{
	"file-list": {},
	"read-list": {},
}

func ParseFile(path string) (*Contract, error) {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	content := string(contentBytes)
	title := parseTitle(content)

	xmlContent, err := normalizeSCMLDocument(content)
	if err != nil {
		return nil, err
	}

	orderedConstants, constantsMap, orderedSections, err := parseNormalizedSCMLDocument(xmlContent)
	if err != nil {
		return nil, err
	}

	if err := resolveConstants(orderedSections, constantsMap); err != nil {
		return nil, err
	}

	sectionsMap := make(map[string][]string, len(orderedSections))
	for _, section := range orderedSections {
		sectionsMap[section.Name] = copyStringSlice(section.Items)
	}

	return &Contract{
		Title:            title,
		Sections:         sectionsMap,
		Constants:        constantsMap,
		SectionRoutes:    buildSectionRoutes(orderedSections),
		OrderedConstants: orderedConstants,
		OrderedSections:  orderedSections,
	}, nil
}

func parseNormalizedSCMLDocument(xmlContent string) ([]ConstantEntry, map[string]string, []SectionEntry, error) {
	decoder := xml.NewDecoder(strings.NewReader(xmlContent))

	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("parse normalized SCML document: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local != "contract" {
				return nil, nil, nil, fmt.Errorf("unexpected root element <%s>", t.Name.Local)
			}
			if err := validateStartElement(t); err != nil {
				return nil, nil, nil, err
			}
			return parseContractElement(decoder, t)
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return nil, nil, nil, fmt.Errorf("non-whitespace content before <contract> is not allowed")
			}
		case xml.Comment, xml.Directive, xml.ProcInst:
			// Ignore.
		default:
			return nil, nil, nil, fmt.Errorf("unsupported token before <contract>: %T", tok)
		}
	}
}

func parseContractElement(decoder *xml.Decoder, start xml.StartElement) ([]ConstantEntry, map[string]string, []SectionEntry, error) {
	orderedConstants := make([]ConstantEntry, 0, 16)
	constantsMap := make(map[string]string)
	orderedSections := make([]SectionEntry, 0, 16)
	sawConstants := false
	sawSection := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("parse <contract>: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "constants":
				if sawConstants {
					return nil, nil, nil, fmt.Errorf("duplicate <constants> block")
				}
				if sawSection {
					return nil, nil, nil, fmt.Errorf("<constants> block must appear before any <section> blocks")
				}
				constants, constantsLookup, err := parseConstantsElement(decoder, t)
				if err != nil {
					return nil, nil, nil, err
				}
				orderedConstants = append(orderedConstants, constants...)
				for k, v := range constantsLookup {
					constantsMap[k] = v
				}
				sawConstants = true
			case "section":
				if !sawConstants {
					return nil, nil, nil, fmt.Errorf("<section> blocks must appear after a complete <constants> block")
				}
				section, err := parseSectionElement(decoder, t, "")
				if err != nil {
					return nil, nil, nil, err
				}
				orderedSections = append(orderedSections, SectionEntry{
					Name:       section.Term,
					DataType:   section.DataType,
					DataPolicy: section.DataPolicy,
					Items:      copyStringSlice(section.Items),
					Routes:     copyRouteNodes(section.Children),
				})
				sawSection = true
			default:
				return nil, nil, nil, fmt.Errorf("unknown XML element <%s> inside <contract>", t.Name.Local)
			}
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return nil, nil, nil, fmt.Errorf("non-whitespace content inside <contract> is not allowed")
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				if !sawConstants {
					return nil, nil, nil, fmt.Errorf("<contract> block must contain a <constants> block")
				}
				if len(orderedSections) == 0 {
					return nil, nil, nil, fmt.Errorf("<contract> block must contain at least one <section>")
				}
				return orderedConstants, constantsMap, orderedSections, nil
			}
		case xml.Comment, xml.Directive, xml.ProcInst:
			// Ignore.
		default:
			return nil, nil, nil, fmt.Errorf("unsupported token inside <contract>: %T", tok)
		}
	}
}

func parseConstantsElement(decoder *xml.Decoder, start xml.StartElement) ([]ConstantEntry, map[string]string, error) {
	ordered := make([]ConstantEntry, 0, 16)
	constants := make(map[string]string)
	sawPre := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, nil, fmt.Errorf("parse <constants>: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pre":
				if err := validateStartElement(t); err != nil {
					return nil, nil, err
				}
				if sawPre {
					return nil, nil, fmt.Errorf("duplicate <pre> block inside <constants>")
				}
				text, err := readTextElement(decoder, t)
				if err != nil {
					return nil, nil, err
				}
				blockOrdered, blockMap, err := parseConstantLines(text)
				if err != nil {
					return nil, nil, err
				}
				ordered = append(ordered, blockOrdered...)
				for k, v := range blockMap {
					constants[k] = v
				}
				sawPre = true
			default:
				return nil, nil, fmt.Errorf("unknown XML element <%s> inside <constants>", t.Name.Local)
			}
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return nil, nil, fmt.Errorf("non-whitespace content inside <constants> is not allowed")
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				if !sawPre {
					return nil, nil, fmt.Errorf("<constants> block must contain a <pre> block")
				}
				return ordered, constants, nil
			}
		case xml.Comment, xml.Directive, xml.ProcInst:
			// Ignore.
		default:
			return nil, nil, fmt.Errorf("unsupported token inside <constants>: %T", tok)
		}
	}
}

func parseSectionElement(decoder *xml.Decoder, start xml.StartElement, parentPath string) (*RouteNode, error) {
	if err := validateStartElement(start); err != nil {
		return nil, err
	}

	term, err := requiredNameAttr(start)
	if err != nil {
		return nil, err
	}
	dataType, err := optionalDataTypeAttr(start)
	if err != nil {
		return nil, err
	}
	dataPolicy, err := optionalDataPolicyAttr(start)
	if err != nil {
		return nil, err
	}
	if !isSectionIdentifier(term) {
		return nil, fmt.Errorf("invalid section name %q: must use letters, digits, underscores, and hyphens only", term)
	}

	term = canonicalSectionName(term)
	pathTerm := canonicalSectionPathTerm(term)
	path := pathTerm
	if parentPath != "" {
		path = parentPath + "." + pathTerm
	}

	node := &RouteNode{
		Term:       term,
		Path:       path,
		DataType:   dataType,
		DataPolicy: dataPolicy,
		Items:      make([]string, 0, 16),
		Children:   make([]RouteNode, 0, 4),
	}

	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("parse <section name=%q>: %w", term, err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "item":
				if err := validateStartElement(t); err != nil {
					return nil, err
				}
				item, err := readTextElement(decoder, t)
				if err != nil {
					return nil, fmt.Errorf("section %q: %w", term, err)
				}
				item = strings.TrimSpace(item)
				if item == "" {
					return nil, fmt.Errorf("section %q: empty item is not allowed", term)
				}
				node.Items = append(node.Items, item)
			case "section":
				child, err := parseSectionElement(decoder, t, path)
				if err != nil {
					return nil, err
				}
				node.Children = append(node.Children, *child)
			default:
				return nil, fmt.Errorf("unknown XML element <%s> inside <section name=%q>", t.Name.Local, term)
			}
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return nil, fmt.Errorf("non-whitespace content inside <section name=%q> is not allowed", term)
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				if len(node.Items) == 0 && len(node.Children) == 0 {
					return nil, fmt.Errorf("section %q must contain item elements or nested sections", term)
				}
				return node, nil
			}
		case xml.Comment, xml.Directive, xml.ProcInst:
			// Ignore.
		default:
			return nil, fmt.Errorf("unsupported token inside <section name=%q>: %T", term, tok)
		}
	}
}

func readTextElement(decoder *xml.Decoder, start xml.StartElement) (string, error) {
	var builder strings.Builder

	for {
		tok, err := decoder.Token()
		if err != nil {
			return "", fmt.Errorf("read <%s> text: %w", start.Name.Local, err)
		}

		switch t := tok.(type) {
		case xml.CharData:
			builder.WriteString(string(t))
		case xml.StartElement:
			return "", fmt.Errorf("nested XML element <%s> is not allowed inside <%s>", t.Name.Local, start.Name.Local)
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return builder.String(), nil
			}
		case xml.Comment, xml.Directive, xml.ProcInst:
			// Ignore.
		default:
			return "", fmt.Errorf("unsupported token inside <%s>: %T", start.Name.Local, tok)
		}
	}
}

func parseConstantLines(content string) ([]ConstantEntry, map[string]string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	ordered := make([]ConstantEntry, 0, 16)
	constants := make(map[string]string)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			return nil, nil, fmt.Errorf("invalid constant line %d: %q", lineNo, line)
		}

		key := strings.TrimSpace(line[:eq])
		valuePart := strings.TrimSpace(line[eq+1:])
		if !isConstantIdentifier(key) {
			return nil, nil, fmt.Errorf("invalid constant key %q", key)
		}
		value, err := strconv.Unquote(valuePart)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid constant value on line %d: %w", lineNo, err)
		}
		if _, exists := constants[key]; exists {
			return nil, nil, fmt.Errorf("duplicate constant key %q", key)
		}
		constants[key] = value
		ordered = append(ordered, ConstantEntry{Key: key, Value: value})
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan constants block: %w", err)
	}

	return ordered, constants, nil
}

func validateStartElement(start xml.StartElement) error {
	name := start.Name.Local
	if name == "item" {
		if len(start.Attr) != 0 {
			return fmt.Errorf("<item> does not allow attributes")
		}
		return nil
	}
	if _, ok := SCMLLanguageConventions.Types[name]; !ok {
		return fmt.Errorf("unknown XML element <%s>", name)
	}

	seenAttrs := make(map[string]struct{}, len(start.Attr))
	for _, attr := range start.Attr {
		if _, ok := SCMLLanguageConventions.Attributes[attr.Name.Local]; !ok {
			return fmt.Errorf("unknown XML attribute %q on <%s>", attr.Name.Local, name)
		}
		if _, exists := seenAttrs[attr.Name.Local]; exists {
			return fmt.Errorf("duplicate XML attribute %q on <%s>", attr.Name.Local, name)
		}
		seenAttrs[attr.Name.Local] = struct{}{}
	}

	switch name {
	case "contract", "constants", "pre":
		if len(start.Attr) != 0 {
			return fmt.Errorf("<%s> does not allow attributes", name)
		}
	case "section":
		if len(start.Attr) < 1 || len(start.Attr) > 3 {
			return fmt.Errorf("<section> requires name and optional data-type/data-policy attributes")
		}
		if _, err := requiredNameAttr(start); err != nil {
			return err
		}
		dataType, err := optionalDataTypeAttr(start)
		if err != nil {
			return err
		}
		if dataType != "" {
			if _, ok := sectionDataTypeValues[dataType]; !ok {
				return fmt.Errorf("invalid section data-type %q", dataType)
			}
		}
		dataPolicy, err := optionalDataPolicyAttr(start)
		if err != nil {
			return err
		}
		if dataPolicy != "" && dataType == "" {
			return fmt.Errorf("data-policy requires data-type")
		}
		if dataPolicy != "" && dataPolicy != "write" {
			return fmt.Errorf("invalid section data-policy %q", dataPolicy)
		}
	}

	return nil
}

func requiredNameAttr(start xml.StartElement) (string, error) {
	for _, attr := range start.Attr {
		if attr.Name.Local == "name" {
			value := strings.TrimSpace(attr.Value)
			if value == "" {
				return "", fmt.Errorf("<section> name attribute cannot be empty")
			}
			return value, nil
		}
	}
	return "", fmt.Errorf("<section> requires a name attribute")
}

func optionalDataTypeAttr(start xml.StartElement) (string, error) {
	for _, attr := range start.Attr {
		if attr.Name.Local == "data-type" {
			value := strings.TrimSpace(attr.Value)
			if value == "" {
				return "", fmt.Errorf("<section> data-type attribute cannot be empty")
			}
			return value, nil
		}
	}
	return "", nil
}

func optionalDataPolicyAttr(start xml.StartElement) (string, error) {
	for _, attr := range start.Attr {
		if attr.Name.Local == "data-policy" {
			value := strings.TrimSpace(attr.Value)
			if value == "" {
				return "", fmt.Errorf("<section> data-policy attribute cannot be empty")
			}
			return value, nil
		}
	}
	return "", nil
}

func isConstantIdentifier(input string) bool {
	if input == "" || unicodeIsDigit(input[0]) {
		return false
	}
	for i := 0; i < len(input); i++ {
		c := input[i]
		if c == '_' || unicodeIsDigit(c) || (c >= 'A' && c <= 'Z') {
			continue
		}
		return false
	}
	return true
}

func isSectionIdentifier(input string) bool {
	if input == "" || unicodeIsDigit(input[0]) {
		return false
	}
	for i := 0; i < len(input); i++ {
		c := input[i]
		if c == '_' || c == '-' || unicodeIsDigit(c) || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			continue
		}
		return false
	}
	return true
}

func canonicalSectionName(input string) string {
	return strings.ToUpper(strings.ReplaceAll(input, "-", "_"))
}

func canonicalSectionPathTerm(input string) string {
	return strings.ToLower(strings.ReplaceAll(input, "-", "_"))
}

func unicodeIsDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func resolveConstants(sections []SectionEntry, constants map[string]string) error {
	for sectionIndex := range sections {
		for itemIndex, item := range sections[sectionIndex].Items {
			resolved, err := replaceConstantRefs(item, constants)
			if err != nil {
				return fmt.Errorf("section %q item %d: %w", sections[sectionIndex].Name, itemIndex, err)
			}
			sections[sectionIndex].Items[itemIndex] = resolved
		}
		if err := resolveConstantsOnRoutes(sections[sectionIndex].Name, sections[sectionIndex].Routes, constants); err != nil {
			return err
		}
	}
	return nil
}

func resolveConstantsOnRoutes(sectionName string, routes []RouteNode, constants map[string]string) error {
	for routeIndex := range routes {
		for itemIndex, item := range routes[routeIndex].Items {
			resolved, err := replaceConstantRefs(item, constants)
			if err != nil {
				return fmt.Errorf("section %q route %q item %d: %w", sectionName, routes[routeIndex].Path, itemIndex, err)
			}
			routes[routeIndex].Items[itemIndex] = resolved
		}
		if err := resolveConstantsOnRoutes(sectionName, routes[routeIndex].Children, constants); err != nil {
			return err
		}
	}
	return nil
}
