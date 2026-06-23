package scml

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func parseTitle(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func isRawXMLLikeLine(line string) bool {
	return strings.HasPrefix(line, "<") && !strings.HasPrefix(line, "<!--")
}

func buildSymbols(inputs []string, fallbackPrefix string) []string {
	used := make(map[string]int, len(inputs))
	out := make([]string, len(inputs))
	for i, in := range inputs {
		base := normalizeSymbol(in, fallbackPrefix)
		count := used[base]
		if count == 0 {
			out[i] = base
			used[base] = 1
			continue
		}
		count++
		used[base] = count
		out[i] = base + strconv.Itoa(count)
	}
	return out
}

func normalizeSymbol(input, fallbackPrefix string) string {
	var builder strings.Builder
	capitalize := true
	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if capitalize {
				builder.WriteRune(unicode.ToUpper(r))
				capitalize = false
			} else {
				builder.WriteRune(unicode.ToLower(r))
			}
			continue
		}
		capitalize = true
	}

	out := builder.String()
	if out == "" {
		out = fallbackPrefix
	}
	if len(out) > 0 {
		first := rune(out[0])
		if unicode.IsDigit(first) {
			out = fallbackPrefix + out
		}
	}
	return out
}

func copyStringSlice(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func copyRouteNodes(in []RouteNode) []RouteNode {
	out := make([]RouteNode, len(in))
	for i := range in {
		out[i] = RouteNode{
			Term:       in[i].Term,
			Path:       in[i].Path,
			DataType:   in[i].DataType,
			DataPolicy: in[i].DataPolicy,
			Items:      copyStringSlice(in[i].Items),
			Children:   copyRouteNodes(in[i].Children),
		}
	}
	return out
}

func copySectionEntry(in SectionEntry) SectionEntry {
	return SectionEntry{
		Name:       in.Name,
		DataType:   in.DataType,
		DataPolicy: in.DataPolicy,
		Items:      copyStringSlice(in.Items),
		Routes:     copyRouteNodes(in.Routes),
	}
}

func toTemplateRoutes(in []RouteNode, symbolState map[string]int) []TemplateRoute {
	out := make([]TemplateRoute, len(in))
	for i := range in {
		symbol := normalizeSymbol(in[i].Term, "Route")
		count := symbolState[symbol]
		if count == 0 {
			symbolState[symbol] = 1
		} else {
			count++
			symbolState[symbol] = count
			symbol = symbol + strconv.Itoa(count)
		}

		out[i] = TemplateRoute{
			Term:       in[i].Term,
			Path:       in[i].Path,
			DataType:   in[i].DataType,
			DataPolicy: in[i].DataPolicy,
			Items:      copyStringSlice(in[i].Items),
			Children:   toTemplateRoutes(in[i].Children, symbolState),
			Symbol:     symbol,
		}
	}
	return out
}

func buildSectionRoutes(sections []SectionEntry) map[string]map[string][]string {
	out := make(map[string]map[string][]string, len(sections))
	for _, section := range sections {
		sectionKey := strings.ToLower(section.Name)
		if _, ok := out[sectionKey]; !ok {
			out[sectionKey] = make(map[string][]string)
		}
		flattenRoutes(out[sectionKey], section.Routes)
	}
	return out
}

func flattenRoutes(out map[string][]string, routes []RouteNode) {
	for _, route := range routes {
		out[route.Path] = copyStringSlice(route.Items)
		flattenRoutes(out, route.Children)
	}
}

func buildRouteTree(children []*routeBuilder) []RouteNode {
	out := make([]RouteNode, len(children))
	for i := range children {
		out[i] = RouteNode{
			Term:     children[i].term,
			Path:     children[i].path,
			Items:    copyStringSlice(children[i].items),
			Children: buildRouteTree(children[i].children),
		}
	}
	return out
}

func replaceConstantRefs(input string, constants map[string]string) (string, error) {
	if !strings.Contains(input, "${") {
		return input, nil
	}

	var builder strings.Builder
	for i := 0; i < len(input); {
		if i+2 < len(input) && input[i] == '$' && input[i+1] == '{' {
			j := i + 2
			for j < len(input) {
				c := input[j]
				if c == '}' {
					break
				}
				if !(c == '_' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
					return "", fmt.Errorf("invalid constant reference near %q", input[i:])
				}
				j++
			}
			if j >= len(input) || input[j] != '}' {
				return "", fmt.Errorf("unterminated constant reference near %q", input[i:])
			}
			key := input[i+2 : j]
			value, ok := constants[key]
			if !ok {
				return "", fmt.Errorf("undefined constant key %q", key)
			}
			builder.WriteString(value)
			i = j + 1
			continue
		}
		builder.WriteByte(input[i])
		i++
	}

	return builder.String(), nil
}
