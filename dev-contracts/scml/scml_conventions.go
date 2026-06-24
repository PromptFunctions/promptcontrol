package scml

var SCMLLanguageConventions = struct {
	Types      map[string]struct{}
	Attributes map[string]struct{}
}{
	Types: map[string]struct{}{
		"scml":      {},
		"constants": {},
		"pre":       {},
		"section":   {},
	},
	Attributes: map[string]struct{}{
		"name":        {},
		"depends-on":  {},
		"data-type":   {},
		"data-policy": {},
		"data-source": {},
	},
}
