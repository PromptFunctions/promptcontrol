package scml

var SCMLLanguageConventions = struct {
	Types      map[string]struct{}
	Attributes map[string]struct{}
}{
	Types: map[string]struct{}{
		"contract":  {},
		"constants": {},
		"pre":       {},
		"section":   {},
	},
	Attributes: map[string]struct{}{
		"name":        {},
		"data-type":   {},
		"data-policy": {},
	},
}
