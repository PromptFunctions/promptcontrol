package scml

import (
	"strings"
	"text/template"
)

const contractTemplate = `# {{ .Title }}

<!-- <scml> -->
<!-- <constants> -->
<pre>
{{- range .Constants }}
{{ .Key }} = "{{ .Value }}"{{- end }}
</pre>
<!-- </constants> -->
{{- range .Sections }}
{{ renderSection . }}{{- end }}
<!-- </scml> -->`

type RenderContract struct {
	Title     string               `json:"Title"`
	Constants []ConstantEntry      `json:"Constants"`
	Sections  []RenderSectionEntry `json:"Sections"`
}

type RenderRouteEntry struct {
	Term       string   `json:"Term"`
	Path       string   `json:"Path"`
	DependsOn  string   `json:"DependsOn,omitempty"`
	DataSource string   `json:"DataSource,omitempty"`
	DataType   string   `json:"DataType,omitempty"`
	DataPolicy string   `json:"DataPolicy,omitempty"`
	Items      []string `json:"Items"`
}

type RenderSectionEntry struct {
	Name       string             `json:"Name"`
	DependsOn  string             `json:"DependsOn,omitempty"`
	DataSource string             `json:"DataSource,omitempty"`
	DataType   string             `json:"DataType,omitempty"`
	DataPolicy string             `json:"DataPolicy,omitempty"`
	Items      []string           `json:"Items,omitempty"`
	Routes     []RenderRouteEntry `json:"Routes,omitempty"`
}

type TemplateConstant struct {
	Key    string
	Value  string
	Symbol string
}

type TemplateRoute struct {
	Term       string
	Path       string
	DependsOn  string
	DataSource string
	DataType   string
	DataPolicy string
	Items      []string
	Children   []TemplateRoute
	Symbol     string
}

type TemplateSection struct {
	Name       string
	DependsOn  string
	DataSource string
	DataType   string
	DataPolicy string
	Items      []string
	Routes     []TemplateRoute
	Symbol     string
}

type TemplateContract struct {
	Title     string
	Constants []TemplateConstant
	Sections  []TemplateSection
}

func (c *Contract) RenderView() RenderContract {
	constants := make([]ConstantEntry, len(c.OrderedConstants))
	copy(constants, c.OrderedConstants)

	sections := make([]RenderSectionEntry, len(c.OrderedSections))
	for i := range c.OrderedSections {
		sections[i] = RenderSectionEntry{
			Name:       c.OrderedSections[i].Name,
			DependsOn:  c.OrderedSections[i].DependsOn,
			DataSource: c.OrderedSections[i].DataSource,
			DataType:   c.OrderedSections[i].DataType,
			DataPolicy: c.OrderedSections[i].DataPolicy,
			Items:      copyStringSlice(c.OrderedSections[i].Items),
			Routes:     flattenRenderRoutes(c.OrderedSections[i].Routes),
		}
	}

	return RenderContract{
		Title:     c.Title,
		Constants: constants,
		Sections:  sections,
	}
}

func flattenRenderRoutes(routes []RouteNode) []RenderRouteEntry {
	out := make([]RenderRouteEntry, 0, countRenderRoutes(routes))
	return appendFlattenedRenderRoutes(out, routes)
}

func appendFlattenedRenderRoutes(out []RenderRouteEntry, routes []RouteNode) []RenderRouteEntry {
	for i := range routes {
		out = append(out, RenderRouteEntry{
			Term:       routes[i].Term,
			Path:       routes[i].Path,
			DependsOn:  routes[i].DependsOn,
			DataSource: routes[i].DataSource,
			DataType:   routes[i].DataType,
			DataPolicy: routes[i].DataPolicy,
			Items:      copyStringSlice(routes[i].Items),
		})
		out = appendFlattenedRenderRoutes(out, routes[i].Children)
	}
	return out
}

func countRenderRoutes(routes []RouteNode) int {
	total := 0
	for i := range routes {
		total++
		total += countRenderRoutes(routes[i].Children)
	}
	return total
}

func (c *Contract) TemplateView() TemplateContract {
	constantNames := make([]string, len(c.OrderedConstants))
	for i := range c.OrderedConstants {
		constantNames[i] = c.OrderedConstants[i].Key
	}
	sectionNames := make([]string, len(c.OrderedSections))
	for i := range c.OrderedSections {
		sectionNames[i] = c.OrderedSections[i].Name
	}

	constantSymbols := buildSymbols(constantNames, "Constant")
	sectionSymbols := buildSymbols(sectionNames, "Section")

	constants := make([]TemplateConstant, len(c.OrderedConstants))
	for i := range c.OrderedConstants {
		constants[i] = TemplateConstant{
			Key:    c.OrderedConstants[i].Key,
			Value:  c.OrderedConstants[i].Value,
			Symbol: constantSymbols[i],
		}
	}

	routeSymbolState := make(map[string]int)
	sections := make([]TemplateSection, len(c.OrderedSections))
	for i := range c.OrderedSections {
		sections[i] = TemplateSection{
			Name:       c.OrderedSections[i].Name,
			DependsOn:  c.OrderedSections[i].DependsOn,
			DataSource: c.OrderedSections[i].DataSource,
			DataType:   c.OrderedSections[i].DataType,
			DataPolicy: c.OrderedSections[i].DataPolicy,
			Items:      copyStringSlice(c.OrderedSections[i].Items),
			Routes:     toTemplateRoutes(c.OrderedSections[i].Routes, routeSymbolState),
			Symbol:     sectionSymbols[i],
		}
	}

	title := strings.TrimSpace(c.Title)
	if title == "" {
		title = "Structured Contract"
	}

	return TemplateContract{
		Title:     title,
		Constants: constants,
		Sections:  sections,
	}
}

func (c *Contract) GoTemplate() string {
	return contractTemplate
}

func TemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"renderSection": renderTemplateSection,
		"renderRoute":   renderTemplateRoute,
	}
}

func renderTemplateSection(section TemplateSection) string {
	var builder strings.Builder
	builder.WriteString("<!-- <section name=\"")
	builder.WriteString(section.Name)
	builder.WriteByte('"')
	if section.DependsOn != "" {
		builder.WriteString(" depends-on=\"")
		builder.WriteString(section.DependsOn)
		builder.WriteByte('"')
	}
	if section.DataSource != "" {
		builder.WriteString(" data-source=\"")
		builder.WriteString(section.DataSource)
		builder.WriteByte('"')
	}
	if section.DataType != "" {
		builder.WriteString(" data-type=\"")
		builder.WriteString(section.DataType)
		builder.WriteByte('"')
	}
	if section.DataPolicy != "" {
		builder.WriteString(" data-policy=\"")
		builder.WriteString(section.DataPolicy)
		builder.WriteByte('"')
	}
	builder.WriteString("> -->\n")
	builder.WriteString("## ")
	builder.WriteString(section.Name)
	builder.WriteString("\n")
	for _, item := range section.Items {
		builder.WriteString("  - ")
		builder.WriteString(item)
		builder.WriteString("\n")
	}
	for _, route := range section.Routes {
		builder.WriteString(renderTemplateRoute(route))
	}
	builder.WriteString("<!-- </section> -->")
	return builder.String()
}

func renderTemplateRoute(route TemplateRoute) string {
	var builder strings.Builder
	builder.WriteString("<!-- <section name=\"")
	builder.WriteString(route.Term)
	builder.WriteByte('"')
	if route.DependsOn != "" {
		builder.WriteString(" depends-on=\"")
		builder.WriteString(route.DependsOn)
		builder.WriteByte('"')
	}
	if route.DataSource != "" {
		builder.WriteString(" data-source=\"")
		builder.WriteString(route.DataSource)
		builder.WriteByte('"')
	}
	if route.DataType != "" {
		builder.WriteString(" data-type=\"")
		builder.WriteString(route.DataType)
		builder.WriteByte('"')
	}
	if route.DataPolicy != "" {
		builder.WriteString(" data-policy=\"")
		builder.WriteString(route.DataPolicy)
		builder.WriteByte('"')
	}
	builder.WriteString("> -->\n")
	for _, item := range route.Items {
		builder.WriteString("  - ")
		builder.WriteString(item)
		builder.WriteString("\n")
	}
	for _, child := range route.Children {
		builder.WriteString(renderTemplateRoute(child))
	}
	builder.WriteString("<!-- </section> -->")
	return builder.String()
}
