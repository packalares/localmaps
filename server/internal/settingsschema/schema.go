// Package settingsschema builds a machine-readable schema for every
// key returned by config.Defaults(). The schema drives both server-side
// validation (see server/internal/api/settings.go) and the
// schema-driven admin settings form (see ui/app/admin/settings).
//
// Pure logic — no SQL, no HTTP, no filesystem. Annotations for ranges,
// enums, and UI groups live alongside in annotations.go and mirror
// docs/07-config-schema.md.
package settingsschema

import (
	"sort"
	"strings"

	"github.com/packalares/localmaps/server/internal/config"
)

// Type is the JSON-schema-ish type tag used by the UI to pick a widget.
type Type string

const (
	TypeString  Type = "string"
	TypeInteger Type = "integer"
	TypeNumber  Type = "number"
	TypeBoolean Type = "boolean"
	TypeEnum    Type = "enum"
	TypeArray   Type = "array"
	TypeObject  Type = "object"
)

// Node is one entry in the schema. Flat: the UI groups by UIGroup and
// orders by the Key's lexical dotted path. The shape is a superset of
// the openapi SettingsSchemaNode — the extra fields (Key, UIGroup,
// ReadOnly, ItemType, Pattern, Unit) are additive and compatible with
// additionalProperties:true on the openapi tree endpoint.
type Node struct {
	Key         string   `json:"key"`
	Type        Type     `json:"type"`
	Description string   `json:"description,omitempty"`
	Default     any      `json:"default"`
	Enum        []any    `json:"enumValues,omitempty"`
	Min         *float64 `json:"minimum,omitempty"`
	Max         *float64 `json:"maximum,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	UIGroup     string   `json:"uiGroup"`
	Unit        string   `json:"unit,omitempty"`
	ItemType    Type     `json:"itemType,omitempty"`
	Step        *float64 `json:"step,omitempty"`
	ReadOnly    bool     `json:"readOnly,omitempty"`
}

// BuildSchema returns one Node per default. The result is stable
// (lexically sorted by key) so the UI form order is deterministic.
// Unknown-annotated keys fall back to a sensible Type inferred from the
// default's Go value; keys declared in annotations but absent from
// defaults are flagged (see ValidateAnnotations).
func BuildSchema(defaults []config.Default) []Node {
	out := make([]Node, 0, len(defaults))
	for _, d := range defaults {
		if d.Key == "schema.version" {
			continue // internal bookkeeping row, never user-visible.
		}
		out = append(out, buildNode(d))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// buildNode synthesises a Node by merging annotations (if any) with the
// inferred type/default of the given Default.
func buildNode(d config.Default) Node {
	n := Node{
		Key:     d.Key,
		Default: d.Value,
		Type:    inferType(d.Value),
		UIGroup: groupOf(d.Key),
	}
	if ann, ok := Annotations[d.Key]; ok {
		applyAnnotation(&n, ann)
	}
	if n.ItemType == "" && n.Type == TypeArray {
		n.ItemType = inferArrayItemType(d.Value)
	}
	return n
}

// applyAnnotation overlays an Annotation entry onto n.
func applyAnnotation(n *Node, a Annotation) {
	if a.Type != "" {
		n.Type = a.Type
	}
	if a.Description != "" {
		n.Description = a.Description
	}
	if len(a.Enum) > 0 {
		n.Enum = append(n.Enum, a.Enum...)
		if n.Type != TypeEnum && n.Type == TypeString {
			n.Type = TypeEnum
		}
	}
	if a.Min != nil {
		v := *a.Min
		n.Min = &v
	}
	if a.Max != nil {
		v := *a.Max
		n.Max = &v
	}
	if a.Step != nil {
		v := *a.Step
		n.Step = &v
	}
	if a.Pattern != "" {
		n.Pattern = a.Pattern
	}
	if a.Unit != "" {
		n.Unit = a.Unit
	}
	if a.UIGroup != "" {
		n.UIGroup = a.UIGroup
	}
	if a.ItemType != "" {
		n.ItemType = a.ItemType
	}
	if a.ReadOnly {
		n.ReadOnly = true
	}
}

// inferType maps a Go default value to a schema Type. Numbers that are
// integral (int / int32 / int64) are reported as TypeInteger; floats and
// any non-integer number become TypeNumber. Slices and maps become
// TypeArray and TypeObject respectively.
func inferType(v any) Type {
	switch v.(type) {
	case bool:
		return TypeBoolean
	case string:
		return TypeString
	case int, int32, int64, uint, uint32, uint64:
		return TypeInteger
	case float32, float64:
		return TypeNumber
	case []string, []any, []int, []map[string]any, []int64:
		return TypeArray
	case map[string]any:
		return TypeObject
	default:
		// Unknown shapes default to object — the UI renders JSON
		// textarea as the fallback widget.
		return TypeObject
	}
}

// inferArrayItemType looks one level into a Go slice for its element
// type, so the UI knows which item widget to render inside a
// FieldStringArray.
func inferArrayItemType(v any) Type {
	switch v.(type) {
	case []string:
		return TypeString
	case []int, []int64:
		return TypeInteger
	case []map[string]any:
		return TypeObject
	default:
		return TypeString
	}
}

// groupOf splits a dotted key into its top-level group.
// map.defaultCenter → "map"; routing.truck.heightMeters → "routing".
func groupOf(key string) string {
	if i := strings.IndexByte(key, '.'); i > 0 {
		return key[:i]
	}
	return key
}

// ByKey returns a lookup map from Key → Node for quick server-side
// validation. The input must come from BuildSchema.
func ByKey(nodes []Node) map[string]Node {
	m := make(map[string]Node, len(nodes))
	for _, n := range nodes {
		m[n.Key] = n
	}
	return m
}

// ValidateAnnotations returns every annotation key that has no matching
// entry in defaults — callers flag it as drift per R2.
func ValidateAnnotations(defaults []config.Default) []string {
	seen := make(map[string]struct{}, len(defaults))
	for _, d := range defaults {
		seen[d.Key] = struct{}{}
	}
	var drift []string
	for k := range Annotations {
		if _, ok := seen[k]; !ok {
			drift = append(drift, k)
		}
	}
	sort.Strings(drift)
	return drift
}
