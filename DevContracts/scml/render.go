package scml

import (
	"strings"
	"text/template"
)

const contractTemplate = `# {{ .Title }}

<!-- <contract> -->
<!-- <constants> -->
<pre>
{{- range .Constants }}
{{ .Key }} = "{{ .Value }}"{{- end }}
</pre>
<!-- </constants> -->
{{- range .Sections }}
{{ renderSection . }}{{- end }}
<!-- </contract> -->`

type RenderContract struct {
	Constants []ConstantEntry `json:"Constants"`
	Sections  []SectionEntry  `json:"Sections"`
}

type TemplateConstant struct {
	Key    string
	Value  string
	Symbol string
}

type TemplateRoute struct {
	Term       string
	Path       string
	DataType   string
	DataPolicy string
	Items      []string
	Children   []TemplateRoute
	Symbol     string
}

type TemplateSection struct {
	Name       string
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

	sections := make([]SectionEntry, len(c.OrderedSections))
	for i := range c.OrderedSections {
		sections[i] = copySectionEntry(c.OrderedSections[i])
	}

	return RenderContract{
		Constants: constants,
		Sections:  sections,
	}
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
