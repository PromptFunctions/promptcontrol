package scml

var allowedSCMLTypes = map[string]struct{}{
	"scml":      {},
	"constants": {},
	"pre":       {},
	"section":   {},
}

var allowedSCMLAttributes = map[string]struct{}{
	"name":        {},
	"depends-on":  {},
	"data-type":   {},
	"data-policy": {},
	"data-source": {},
}

func AllowedSCMLTypes() []string {
	return copySortedKeys(allowedSCMLTypes)
}

func AllowedSCMLAttributes() []string {
	return copySortedKeys(allowedSCMLAttributes)
}
