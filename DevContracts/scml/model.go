package scml

type ConstantEntry struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type RouteNode struct {
	Term       string      `json:"Term"`
	Path       string      `json:"Path"`
	DataType   string      `json:"DataType,omitempty"`
	DataPolicy string      `json:"DataPolicy,omitempty"`
	Items      []string    `json:"Items"`
	Children   []RouteNode `json:"Children"`
}

type SectionEntry struct {
	Name       string      `json:"Name"`
	DataType   string      `json:"DataType,omitempty"`
	DataPolicy string      `json:"DataPolicy,omitempty"`
	Items      []string    `json:"Items,omitempty"`
	Routes     []RouteNode `json:"Routes,omitempty"`
}

type Contract struct {
	Title string

	Sections      map[string][]string
	Constants     map[string]string
	SectionRoutes map[string]map[string][]string

	OrderedConstants []ConstantEntry
	OrderedSections  []SectionEntry
}

type routeBuilder struct {
	term      string
	path      string
	dataType  string
	items     []string
	children  []*routeBuilder
	seenChild map[string]struct{}
}
