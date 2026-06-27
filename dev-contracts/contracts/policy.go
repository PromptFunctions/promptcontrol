package scml

import "strings"

const (
	PolicyActionRead  = "read"
	PolicyActionWrite = "write"
)

type AccessPolicy struct {
	ReadAllowlist   []string            `json:"ReadAllowlist,omitempty"`
	WriteAllowlist  []string            `json:"WriteAllowlist,omitempty"`
	PermissionTable []PermissionEntry   `json:"PermissionTable,omitempty"`
	Rules           map[string][]string `json:"Rules,omitempty"`
}

type PermissionEntry struct {
	Path  string `json:"Path"`
	Write bool   `json:"Write"`
}

func (c *Contract) PolicyView() AccessPolicy {
	policy := AccessPolicy{
		ReadAllowlist:   make([]string, 0, 16),
		WriteAllowlist:  make([]string, 0, 16),
		PermissionTable: make([]PermissionEntry, 0, 16),
		Rules:           map[string][]string{PolicyActionRead: []string{}, PolicyActionWrite: []string{}},
	}

	seenRead := make(map[string]struct{})
	seenWrite := make(map[string]struct{})
	seenPermissions := make(map[string]struct{})

	for _, section := range c.OrderedSections {
		collectPolicyItems(section.DataType, section.DataPolicy, section.Items, &policy, seenRead, seenWrite, seenPermissions)
		collectPolicyRoutes(section.Routes, &policy, seenRead, seenWrite, seenPermissions)
	}

	policy.Rules[PolicyActionRead] = copyStringSlice(policy.ReadAllowlist)
	policy.Rules[PolicyActionWrite] = copyStringSlice(policy.WriteAllowlist)
	return policy
}

func (p AccessPolicy) Allows(action, target string) bool {
	normalizedTarget := normalizePolicyPath(target)
	switch strings.ToLower(strings.TrimSpace(action)) {
	case PolicyActionRead:
		return containsPolicyPath(p.ReadAllowlist, normalizedTarget)
	case PolicyActionWrite:
		return containsPolicyPath(p.WriteAllowlist, normalizedTarget)
	default:
		return false
	}
}

func (p AccessPolicy) Allowlist(action string) []string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case PolicyActionRead:
		return copyStringSlice(p.ReadAllowlist)
	case PolicyActionWrite:
		return copyStringSlice(p.WriteAllowlist)
	default:
		return nil
	}
}

func collectPolicyRoutes(routes []RouteNode, policy *AccessPolicy, seenRead, seenWrite, seenPermissions map[string]struct{}) {
	for _, route := range routes {
		collectPolicyItems(route.DataType, route.DataPolicy, route.Items, policy, seenRead, seenWrite, seenPermissions)
		collectPolicyRoutes(route.Children, policy, seenRead, seenWrite, seenPermissions)
	}
}

func collectPolicyItems(dataType, dataPolicy string, items []string, policy *AccessPolicy, seenRead, seenWrite, seenPermissions map[string]struct{}) {
	switch dataType {
	case "read-list":
		for _, item := range items {
			addPolicyPath(&policy.ReadAllowlist, seenRead, item)
		}
	case "file-list":
		if dataPolicy != "write" {
			return
		}
		for _, item := range items {
			addPolicyPath(&policy.WriteAllowlist, seenWrite, item)
			addPermissionEntry(&policy.PermissionTable, seenPermissions, item, true)
		}
	}
}

func addPolicyPath(out *[]string, seen map[string]struct{}, raw string) {
	cleaned := normalizePolicyPath(raw)
	if cleaned == "" {
		return
	}
	if _, ok := seen[cleaned]; ok {
		return
	}
	seen[cleaned] = struct{}{}
	*out = append(*out, cleaned)
}

func addPermissionEntry(out *[]PermissionEntry, seen map[string]struct{}, raw string, write bool) {
	cleaned := normalizePolicyPath(raw)
	if cleaned == "" {
		return
	}
	if _, ok := seen[cleaned]; ok {
		return
	}
	seen[cleaned] = struct{}{}
	*out = append(*out, PermissionEntry{Path: cleaned, Write: write})
}

func normalizePolicyPath(path string) string {
	return cleanPath(path)
}

func containsPolicyPath(paths []string, target string) bool {
	for _, path := range paths {
		if normalizePolicyPath(path) == target {
			return true
		}
	}
	return false
}
