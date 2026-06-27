package scml

import "fmt"

const (
	RenderValidationModeFixedSlots      = "fixed-slots"
	RenderValidationModeOpenCardinality = "open-cardinality"
)

type RenderValidationFileList struct {
	ItemsPath string
	BaseDir   string
	Mode      string
}

type RenderValidation struct {
	FileLists []RenderValidationFileList
}

func (c *Contract) RenderValidation() RenderValidation {
	_, fileLists := buildRenderedSections(c.OrderedSections)
	return RenderValidation{FileLists: fileLists}
}

func buildRenderedSections(sections []SectionEntry) ([]RenderSectionEntry, []RenderValidationFileList) {
	rendered := make([]RenderSectionEntry, len(sections))
	fileLists := make([]RenderValidationFileList, 0, 8)
	for i := range sections {
		sectionPath := fmt.Sprintf("Sections[%d]", i)
		rendered[i] = RenderSectionEntry{
			Name:       sections[i].Name,
			DependsOn:  sections[i].DependsOn,
			DataSource: sections[i].DataSource,
			DataType:   sections[i].DataType,
			DataPolicy: sections[i].DataPolicy,
			Items:      copyStringSlice(sections[i].Items),
		}
		if sections[i].DataType == "file-list" {
			fileLists = append(fileLists, RenderValidationFileList{
				ItemsPath: sectionPath + ".Items",
				BaseDir:   sections[i].sourceDir,
				Mode:      RenderValidationModeOpenCardinality,
			})
		}
		rendered[i].Routes, fileLists, _ = appendRenderedRoutes(rendered[i].Routes, fileLists, sections[i].Routes, sectionPath+".Routes", 0)
	}
	return rendered, fileLists
}

func appendRenderedRoutes(rendered []RenderRouteEntry, fileLists []RenderValidationFileList, routes []RouteNode, parentPath string, start int) ([]RenderRouteEntry, []RenderValidationFileList, int) {
	index := start
	for i := range routes {
		routePath := fmt.Sprintf("%s[%d]", parentPath, index)
		index++
		route := routes[i]
		rendered = append(rendered, RenderRouteEntry{
			Term:       route.Term,
			Path:       route.Path,
			DependsOn:  route.DependsOn,
			DataSource: route.DataSource,
			DataType:   route.DataType,
			DataPolicy: route.DataPolicy,
			Items:      copyStringSlice(route.Items),
		})
		if route.DataType == "file-list" {
			fileLists = append(fileLists, RenderValidationFileList{
				ItemsPath: routePath + ".Items",
				BaseDir:   route.sourceDir,
				Mode:      RenderValidationModeOpenCardinality,
			})
		}
		var next int
		rendered, fileLists, next = appendRenderedRoutes(rendered, fileLists, route.Children, parentPath, index)
		index = next
	}
	return rendered, fileLists, index
}
