/*
Prompt Control (Standalone Fixture)
===================================

What this file is:
- A stateless contract-enforcement engine for JSON outputs produced by LLMs.
- A structural reference implementation that can be embedded in agentic workflows.

Why Bloom filters are used:
- They provide fast membership checks with low memory overhead.
- They scale well when validating many contract keys repeatedly.

Important architecture decision:
- Bloom filters can have false positives.
- For deterministic correctness, this implementation uses:
 1. Bloom filter as a fast pre-check
 2. Exact map membership as the authoritative check

Anatomy:
1) Flatten the contract shape into canonical nested key paths.
2) Flatten the candidate JSON into canonical nested key paths.
3) Build Bloom filter + exact key map from contract keys.
4) Detect missing keys deterministically.
5) Return:
  - "completed" when no key is missing
  - "missing_fields" with a sorted list otherwise
*/
package promptcontrol

import (
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
	"strings"
)

const (
	JSONContractCompletedStatus = "completed"
	JSONContractMissingStatus   = "missing_fields"
)

type bloomFilter struct {
	size      uint64
	hashCount uint64
	bits      []bool
}

func newBloomFilter(size, hashCount uint64) *bloomFilter {
	if size == 0 {
		size = 20000
	}
	if hashCount == 0 {
		hashCount = 7
	}
	return &bloomFilter{
		size:      size,
		hashCount: hashCount,
		bits:      make([]bool, size),
	}
}

func (b *bloomFilter) add(key string) {
	for _, idx := range b.indexes(key) {
		b.bits[idx] = true
	}
}

func (b *bloomFilter) contains(key string) bool {
	for _, idx := range b.indexes(key) {
		if !b.bits[idx] {
			return false
		}
	}
	return true
}

func (b *bloomFilter) indexes(key string) []uint64 {
	out := make([]uint64, 0, b.hashCount)
	for i := uint64(0); i < b.hashCount; i++ {
		h := fnv.New64a()
		_, _ = h.Write([]byte(key))
		_, _ = h.Write([]byte{':'})
		_, _ = h.Write([]byte(fmt.Sprintf("%d", i)))
		out = append(out, h.Sum64()%b.size)
	}
	return out
}

// JSONContract validates jsonToValidate against contractStruct.
// Returns:
// - ("completed", nil) when all expected keys exist.
// - ("missing_fields", []missingKeys) otherwise.
func JSONContract(jsonToValidate map[string]any, contractStruct any) (string, []string) {
	contractKeys := flattenContractKeys(contractStruct)
	candidateKeys := flattenJSONKeys(jsonToValidate)

	contractMap := make(map[string]struct{}, len(contractKeys))
	filter := newBloomFilter(20000, 7)
	for _, key := range contractKeys {
		contractMap[key] = struct{}{}
		filter.add(key)
	}

	missing := make([]string, 0, len(contractKeys))
	for _, key := range contractKeys {
		if _, ok := candidateKeys[key]; ok {
			continue
		}

		// Fast probabilistic pre-check.
		// Authoritative result still comes from exact key map logic.
		_ = filter.contains(key)
		if _, ok := contractMap[key]; ok {
			missing = append(missing, key)
		}
	}

	sort.Strings(missing)
	if len(missing) == 0 {
		return JSONContractCompletedStatus, nil
	}
	return JSONContractMissingStatus, missing
}

func flattenContractKeys(contractStruct any) []string {
	keys := make([]string, 0, 32)
	v := reflect.ValueOf(contractStruct)
	if !v.IsValid() {
		return keys
	}

	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v = reflect.New(v.Type().Elem()).Elem()
			break
		}
		v = v.Elem()
	}
	flattenReflectValue(v, "", &keys)
	sort.Strings(keys)
	return dedupeSorted(keys)
}

func flattenReflectValue(v reflect.Value, prefix string, keys *[]string) {
	if !v.IsValid() {
		return
	}

	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := parseJSONFieldName(field.Tag.Get("json"), field.Name)
			if name == "-" || name == "" {
				continue
			}
			path := joinPath(prefix, name)
			*keys = append(*keys, path)
			flattenReflectValue(v.Field(i), path, keys)
		}
	case reflect.Map:
		// Dynamic maps are not expanded (unknown key universe).
		return
	default:
		return
	}
}

func parseJSONFieldName(tag, fallback string) string {
	if tag == "" {
		return lowerFirst(fallback)
	}
	name := strings.Split(tag, ",")[0]
	if name == "" {
		return lowerFirst(fallback)
	}
	return name
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	if len(s) == 1 {
		return strings.ToLower(s)
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func flattenJSONKeys(data map[string]any) map[string]struct{} {
	out := make(map[string]struct{}, 32)
	var walk func(prefix string, v any)

	walk = func(prefix string, v any) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		for key, child := range m {
			path := joinPath(prefix, key)
			out[path] = struct{}{}
			walk(path, child)
		}
	}

	walk("", data)
	return out
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func dedupeSorted(values []string) []string {
	if len(values) == 0 {
		return values
	}
	out := values[:1]
	for i := 1; i < len(values); i++ {
		if values[i] == values[i-1] {
			continue
		}
		out = append(out, values[i])
	}
	return out
}
