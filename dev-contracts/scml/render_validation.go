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
	fileLists := make([]RenderValidationFileList, 0, 8)
	for i, section := range c.OrderedSections {
		sectionPath := fmt.Sprintf("Sections[%d]", i)
		if section.DataType == "file-list" {
			fileLists = append(fileLists, RenderValidationFileList{
				ItemsPath: sectionPath + ".Items",
				BaseDir:   section.sourceDir,
				Mode:      RenderValidationModeOpenCardinality,
			})
		}
		fileLists, _ = appendFlattenedRenderValidationFileLists(fileLists, section.Routes, sectionPath+".Routes", 0)
	}
	return RenderValidation{FileLists: fileLists}
}

func appendFlattenedRenderValidationFileLists(fileLists []RenderValidationFileList, routes []RouteNode, parentPath string, start int) ([]RenderValidationFileList, int) {
	index := start
	for i := range routes {
		routePath := fmt.Sprintf("%s[%d]", parentPath, index)
		index++
		route := routes[i]
		if route.DataType == "file-list" {
			fileLists = append(fileLists, RenderValidationFileList{
				ItemsPath: routePath + ".Items",
				BaseDir:   route.sourceDir,
				Mode:      RenderValidationModeOpenCardinality,
			})
		}
		var errIndex int
		fileLists, errIndex = appendFlattenedRenderValidationFileLists(fileLists, route.Children, parentPath, index)
		index = errIndex
	}
	return fileLists, index
}
