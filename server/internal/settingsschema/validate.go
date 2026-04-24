package settingsschema

import (
	"fmt"
	"regexp"
)

// ValidateValue checks that v satisfies the constraints on n. It is the
// single source of truth for runtime-setting validation on both the
// GET (sanity on read) and PATCH (reject invalid writes) paths.
//
// It returns a user-safe error message for the first failure. Callers
// log with zerolog and surface the message to clients via apierr.
func ValidateValue(n Node, v any) error {
	switch n.Type {
	case TypeBoolean:
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("%s: expected boolean", n.Key)
		}
	case TypeString:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s: expected string", n.Key)
		}
		if n.Pattern != "" {
			re, err := regexp.Compile(n.Pattern)
			if err == nil && !re.MatchString(s) {
				return fmt.Errorf("%s: value does not match pattern %s", n.Key, n.Pattern)
			}
		}
	case TypeEnum:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s: expected enum string", n.Key)
		}
		for _, e := range n.Enum {
			if es, ok := e.(string); ok && es == s {
				return nil
			}
		}
		return fmt.Errorf("%s: %q is not an allowed value", n.Key, s)
	case TypeInteger:
		f, ok := asFloat(v)
		if !ok {
			return fmt.Errorf("%s: expected integer", n.Key)
		}
		if f != float64(int64(f)) {
			return fmt.Errorf("%s: expected integer, got fractional value", n.Key)
		}
		if err := rangeCheck(n, f); err != nil {
			return err
		}
	case TypeNumber:
		f, ok := asFloat(v)
		if !ok {
			return fmt.Errorf("%s: expected number", n.Key)
		}
		if err := rangeCheck(n, f); err != nil {
			return err
		}
	case TypeArray:
		arr, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s: expected array", n.Key)
		}
		for i, item := range arr {
			switch n.ItemType {
			case TypeString:
				if _, ok := item.(string); !ok {
					return fmt.Errorf("%s[%d]: expected string", n.Key, i)
				}
			case TypeInteger:
				if f, ok := asFloat(item); !ok || f != float64(int64(f)) {
					return fmt.Errorf("%s[%d]: expected integer", n.Key, i)
				}
			case TypeObject:
				if _, ok := item.(map[string]any); !ok {
					return fmt.Errorf("%s[%d]: expected object", n.Key, i)
				}
			}
		}
	case TypeObject:
		if _, ok := v.(map[string]any); !ok {
			return fmt.Errorf("%s: expected object", n.Key)
		}
	}
	return nil
}

// asFloat coerces a JSON-decoded number into float64. encoding/json
// decodes every number into float64, but callers may pass ints/int64.
func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	}
	return 0, false
}

func rangeCheck(n Node, f float64) error {
	if n.Min != nil && f < *n.Min {
		return fmt.Errorf("%s: %v below minimum %v", n.Key, f, *n.Min)
	}
	if n.Max != nil && f > *n.Max {
		return fmt.Errorf("%s: %v above maximum %v", n.Key, f, *n.Max)
	}
	return nil
}
