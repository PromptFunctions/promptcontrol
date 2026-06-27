package forge

import (
	"fmt"
	"strings"

	scml "github.com/PromptFunctions/promptcontrol/dev-contracts/contracts"
)

const llmFillPrefix = "[LLM TO FILL INSTRUCTIONS: "

func buildReferenceShape(reference scml.RenderContract) map[string]any {
	return buildRenderShapeMap(reference, nil, false)
}

func buildAttemptScaffold(reference scml.RenderContract, working map[string]any) map[string]any {
	return buildRenderShapeMap(reference, working, true)
}

func buildPromptScaffold(reference scml.RenderContract, checkpoint map[string]any) map[string]any {
	shape := map[string]any{
		"Title":     reference.Title,
		"Constants": make([]any, len(reference.Constants)),
		"Sections":  make([]any, len(reference.Sections)),
	}

	constants := shape["Constants"].([]any)
	for i := range reference.Constants {
		constants[i] = map[string]any{
			"Key":   reference.Constants[i].Key,
			"Value": reference.Constants[i].Value,
		}
	}

	sections := shape["Sections"].([]any)
	for i := range reference.Sections {
		sections[i] = buildPromptSectionShape(reference.Sections[i], i, checkpoint)
	}
	return shape
}

func buildRenderShapeMap(reference scml.RenderContract, working map[string]any, blankMissing bool) map[string]any {
	shape := map[string]any{
		"Title":     reference.Title,
		"Constants": make([]any, len(reference.Constants)),
		"Sections":  make([]any, len(reference.Sections)),
	}

	constants := shape["Constants"].([]any)
	for i := range reference.Constants {
		constants[i] = map[string]any{
			"Key":   reference.Constants[i].Key,
			"Value": reference.Constants[i].Value,
		}
	}

	sections := shape["Sections"].([]any)
	for i := range reference.Sections {
		sections[i] = buildSectionShape(reference.Sections[i], i, working, blankMissing)
	}
	return shape
}

func buildPromptSectionShape(section scml.RenderSectionEntry, index int, checkpoint map[string]any) map[string]any {
	shape := map[string]any{
		"Name":       section.Name,
		"DataSource": section.DataSource,
		"DataType":   section.DataType,
		"DataPolicy": section.DataPolicy,
		"Items":      buildPromptStringSlots(fmt.Sprintf("Sections[%d].Items", index), section.DataType, section.Items, checkpoint),
		"Routes":     make([]any, len(section.Routes)),
	}

	routes := shape["Routes"].([]any)
	for i := range section.Routes {
		routes[i] = buildPromptRouteShape(section.Routes[i], index, i, checkpoint)
	}
	return shape
}

func buildSectionShape(section scml.RenderSectionEntry, index int, working map[string]any, blankMissing bool) map[string]any {
	shape := map[string]any{
		"Name":       section.Name,
		"DataSource": section.DataSource,
		"DataType":   section.DataType,
		"DataPolicy": section.DataPolicy,
		"Items":      buildStringSlots(fmt.Sprintf("Sections[%d].Items", index), section.DataType, section.Items, working, blankMissing),
		"Routes":     make([]any, len(section.Routes)),
	}

	routes := shape["Routes"].([]any)
	for i := range section.Routes {
		routes[i] = buildRouteShape(section.Routes[i], index, i, working, blankMissing)
	}
	return shape
}

func buildPromptRouteShape(route scml.RenderRouteEntry, sectionIndex, routeIndex int, checkpoint map[string]any) map[string]any {
	return map[string]any{
		"Term":       route.Term,
		"Path":       route.Path,
		"DataSource": route.DataSource,
		"DataType":   route.DataType,
		"DataPolicy": route.DataPolicy,
		"Items": buildPromptStringSlots(
			fmt.Sprintf("Sections[%d].Routes[%d].Items", sectionIndex, routeIndex),
			route.DataType,
			route.Items,
			checkpoint,
		),
	}
}

func buildRouteShape(route scml.RenderRouteEntry, sectionIndex, routeIndex int, working map[string]any, blankMissing bool) map[string]any {
	return map[string]any{
		"Term":       route.Term,
		"Path":       route.Path,
		"DataSource": route.DataSource,
		"DataType":   route.DataType,
		"DataPolicy": route.DataPolicy,
		"Items": buildStringSlots(
			fmt.Sprintf("Sections[%d].Routes[%d].Items", sectionIndex, routeIndex),
			route.DataType,
			route.Items,
			working,
			blankMissing,
		),
	}
}

func buildPromptStringSlots(path, dataType string, referenceItems []string, checkpoint map[string]any) []any {
	if dataType == "file-list" {
		if items, ok := lookupStringSlicePath(checkpoint, path); ok {
			return items
		}
		return []any{}
	}

	items := make([]any, len(referenceItems))
	for i := range referenceItems {
		value, ok := lookupStringPath(checkpoint, fmt.Sprintf("%s[%d]", path, i))
		if ok && strings.TrimSpace(value) != "" {
			items[i] = value
			continue
		}
		items[i] = promptPlaceholder(referenceItems[i])
	}
	return items
}

func buildStringSlots(path, dataType string, referenceItems []string, working map[string]any, blankMissing bool) []any {
	if blankMissing && dataType == "file-list" {
		if items, ok := lookupStringSlicePath(working, path); ok {
			return items
		}
		return []any{}
	}

	items := make([]any, len(referenceItems))
	for i := range referenceItems {
		if !blankMissing {
			items[i] = referenceItems[i]
			continue
		}
		value, ok := lookupStringPath(working, fmt.Sprintf("%s[%d]", path, i))
		if ok && strings.TrimSpace(value) != "" {
			items[i] = value
			continue
		}
		items[i] = ""
	}
	return items
}

func promptPlaceholder(value string) string {
	return llmFillPrefix + value + "]"
}

func lookupStringSlicePath(root map[string]any, path string) ([]any, bool) {
	if len(root) == 0 {
		return nil, false
	}
	value, ok := readJSONPath(root, path)
	if !ok {
		return nil, false
	}
	rawItems, ok := value.([]any)
	if !ok {
		return nil, false
	}
	items := make([]any, 0, len(rawItems))
	for _, raw := range rawItems {
		text, ok := raw.(string)
		if !ok {
			return nil, false
		}
		items = append(items, text)
	}
	return items, true
}

func lookupStringPath(root map[string]any, path string) (string, bool) {
	if len(root) == 0 {
		return "", false
	}
	value, ok := readJSONPath(root, path)
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return text, true
}
