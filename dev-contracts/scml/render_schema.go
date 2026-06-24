package scml

func (c *Contract) RenderSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"Title", "Constants", "Sections"},
		"properties": map[string]any{
			"Title":     stringSchema(),
			"Constants": constantArraySchema(),
			"Sections":  sectionArraySchema(),
		},
	}
}

func constantArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"Key", "Value"},
			"properties": map[string]any{
				"Key":   stringSchema(),
				"Value": stringSchema(),
			},
		},
	}
}

func sectionArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"Name", "DependsOn", "DataSource", "DataType", "DataPolicy", "Items", "Routes"},
			"properties": map[string]any{
				"Name":       stringSchema(),
				"DependsOn":  stringSchema(),
				"DataSource": stringSchema(),
				"DataType":   stringSchema(),
				"DataPolicy": stringSchema(),
				"Items":      stringArraySchema(),
				"Routes":     routeArraySchema(),
			},
		},
	}
}

func routeArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"Term", "Path", "DependsOn", "DataSource", "DataType", "DataPolicy", "Items"},
			"properties": map[string]any{
				"Term":       stringSchema(),
				"Path":       stringSchema(),
				"DependsOn":  stringSchema(),
				"DataSource": stringSchema(),
				"DataType":   stringSchema(),
				"DataPolicy": stringSchema(),
				"Items":      stringArraySchema(),
			},
		},
	}
}

func stringSchema() map[string]any {
	return map[string]any{"type": "string"}
}

func stringArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "string",
		},
	}
}
