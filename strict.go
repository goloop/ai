package ai

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ValidateStrictSchema reports whether a schema can be enforced strictly as
// written, by the rules providers that offer strict structured output share:
// every object that declares properties must also set additionalProperties to
// false and list every one of those properties in required.
//
// It exists because a schema is a literal, fully known before the first call,
// and the alternative to checking it here is learning about a typo from a 400
// on a live key after a deploy. It walks properties, items, prefixItems,
// $defs/definitions and anyOf/oneOf/allOf, so a mistake nested three levels
// down is reported with the path that leads to it:
//
//	properties.stories.items: "required" is missing "placeLocal"
//
// It is deliberately not part of [Request.Validate]: strictness is a provider
// dialect, and a provider-agnostic validator has no business enforcing one.
// Drivers whose provider needs these rules call it themselves when
// [Format.Strict] is set; anyone assembling schemas can call it in a test.
//
// This is not a JSON Schema validator. It checks the rules that decide whether
// strict mode is accepted, and says nothing about whether the schema describes
// what you meant.
func ValidateStrictSchema(schema json.RawMessage) error {
	if len(schema) == 0 {
		return ErrNoSchema
	}
	var node any
	if err := json.Unmarshal(schema, &node); err != nil {
		return fmt.Errorf("%w: %v", ErrBadSchema, err)
	}
	if _, ok := node.(map[string]any); !ok {
		return ErrBadSchema
	}
	return checkStrict(node, "")
}

// checkStrict walks one schema node. The path names where a failure is, in the
// dotted form a reader can follow back into their own literal.
func checkStrict(node any, path string) error {
	obj, ok := node.(map[string]any)
	if !ok {
		return nil
	}

	// A node that declares properties is an object schema, whatever its
	// "type" says, and it is exactly what the strict rules are about.
	if props, ok := obj["properties"].(map[string]any); ok {
		if err := checkStrictObject(obj, props, path); err != nil {
			return err
		}
		for name, sub := range props {
			if err := checkStrict(sub, join(path, "properties."+name)); err != nil {
				return err
			}
		}
	}

	// Everywhere else a schema can hide another schema.
	for _, key := range []string{"items", "additionalItems", "not"} {
		if sub, ok := obj[key]; ok {
			if err := checkStrict(sub, join(path, key)); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"prefixItems", "anyOf", "oneOf", "allOf"} {
		list, ok := obj[key].([]any)
		if !ok {
			continue
		}
		for i, sub := range list {
			p := join(path, fmt.Sprintf("%s[%d]", key, i))
			if err := checkStrict(sub, p); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"$defs", "definitions"} {
		defs, ok := obj[key].(map[string]any)
		if !ok {
			continue
		}
		for name, sub := range defs {
			if err := checkStrict(sub, join(path, key+"."+name)); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkStrictObject applies the two rules to one object schema.
func checkStrictObject(obj, props map[string]any, path string) error {
	if extra, ok := obj["additionalProperties"]; !ok || extra != false {
		return fmt.Errorf("%w: %s\"additionalProperties\" must be false",
			ErrBadStrictSchema, prefix(path))
	}

	required := map[string]bool{}
	if list, ok := obj["required"].([]any); ok {
		for _, r := range list {
			if name, ok := r.(string); ok {
				required[name] = true
			}
		}
	}

	var missing []string
	for name := range props {
		if !required[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		// Map order is random; a stable message is what makes the error
		// worth putting in a test.
		sort.Strings(missing)
		return fmt.Errorf("%w: %s\"required\" is missing %s",
			ErrBadStrictSchema, prefix(path), quoteAll(missing))
	}

	return nil
}

// join extends a schema path with one more step.
func join(path, step string) string {
	if path == "" {
		return step
	}
	return path + "." + step
}

// prefix renders a path for an error message, or nothing at the root.
func prefix(path string) string {
	if path == "" {
		return ""
	}
	return path + ": "
}

// quoteAll renders names as a readable list.
func quoteAll(names []string) string {
	var b strings.Builder
	for i, n := range names {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`"` + n + `"`)
	}
	return b.String()
}
