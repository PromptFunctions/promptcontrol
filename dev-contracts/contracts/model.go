package scml

type ConstantEntry struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type ImportEntry struct {
	Path string `json:"Path"`
	Name string `json:"Name,omitempty"`
}

type RouteNode struct {
	Term       string      `json:"Term"`
	Path       string      `json:"Path"`
	DependsOn  string      `json:"DependsOn,omitempty"`
	DataSource string      `json:"DataSource,omitempty"`
	DataType   string      `json:"DataType,omitempty"`
	DataPolicy string      `json:"DataPolicy,omitempty"`
	Items      []string    `json:"Items"`
	Children   []RouteNode `json:"Children"`
	sourceDir  string
}

type SectionEntry struct {
	Name       string      `json:"Name"`
	DependsOn  string      `json:"DependsOn,omitempty"`
	DataSource string      `json:"DataSource,omitempty"`
	DataType   string      `json:"DataType,omitempty"`
	DataPolicy string      `json:"DataPolicy,omitempty"`
	Items      []string    `json:"Items,omitempty"`
	Routes     []RouteNode `json:"Routes,omitempty"`
	sourceDir  string
}

type Contract struct {
	Title string

	Imports []ImportEntry

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
