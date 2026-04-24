package settingsschema

import "strings"

// Tree nests a flat `key.subkey.leaf` → value map under a dotted-path
// tree, matching the openapi SettingsTree contract (`additionalProperties:true`).
// Collision (e.g. "map" has a value AND "map.style") is resolved by
// letting the deeper key win — in practice the settings store never
// has a parent row and its child simultaneously.
func Tree(flat map[string]any) map[string]any {
	root := map[string]any{}
	for k, v := range flat {
		parts := strings.Split(k, ".")
		cur := root
		for i, p := range parts {
			if i == len(parts)-1 {
				cur[p] = v
				continue
			}
			next, ok := cur[p].(map[string]any)
			if !ok {
				next = map[string]any{}
				cur[p] = next
			}
			cur = next
		}
	}
	return root
}

// Flatten walks a nested tree and returns a flat `dotted.key` → value
// map. Leaves are anything that is not a `map[string]any`.
func Flatten(tree map[string]any) map[string]any {
	out := map[string]any{}
	var walk func(prefix string, node map[string]any)
	walk = func(prefix string, node map[string]any) {
		for k, v := range node {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			if sub, ok := v.(map[string]any); ok {
				// Special-case: known object-typed leaves (like
				// map.defaultCenter) contain primitive keys ("lat",
				// "lon", "zoom") that would otherwise be flattened.
				// We treat a map whose values are all primitives as
				// a leaf if the caller has registered it as such via
				// IsObjectLeaf.
				if IsObjectLeaf(key) {
					out[key] = v
				} else {
					walk(key, sub)
				}
				continue
			}
			out[key] = v
		}
	}
	walk("", tree)
	return out
}

// objectLeafKeys lists dotted paths whose value is a JSON object the
// settings store writes as a single row rather than as a nested subtree.
// Keep in sync with any future map[string]any defaults.
var objectLeafKeys = map[string]struct{}{
	"map.defaultCenter": {},
}

// IsObjectLeaf reports whether a dotted key must be treated as a single
// object leaf (not deep-merged).
func IsObjectLeaf(key string) bool {
	_, ok := objectLeafKeys[key]
	return ok
}
