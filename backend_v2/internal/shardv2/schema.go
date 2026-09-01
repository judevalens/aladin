package shardv2

import (
	"container/list"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
)

// DecodeJSON bounds attacker-controlled documents before a schema engine sees
// them. Numeric policy deliberately matches JavaScript's JSON number domain.
func DecodeJSON(data []byte) (any, error) {
	if len(data) > MaxJSONBytes {
		return nil, fmt.Errorf("JSON exceeds %d bytes", MaxJSONBytes)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	if err := SafeJSON(value, 0); err != nil {
		return nil, err
	}
	return value, nil
}

func SafeJSON(v any, depth int) error {
	if depth > MaxJSONDepth {
		return fmt.Errorf("JSON depth exceeds %d", MaxJSONDepth)
	}
	switch x := v.(type) {
	case nil, string, bool:
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) || (math.Trunc(x) == x && math.Abs(x) > 9007199254740991) {
			return fmt.Errorf("unsafe JSON number")
		}
	case []any:
		for _, item := range x {
			if err := SafeJSON(item, depth+1); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range x {
			if err := SafeJSON(item, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("not a JSON value: %T", v)
	}
	return nil
}

var resolvedProtocols sync.Map

// Compiled validators are immutable. Bound dynamic schemas by both count and
// source bytes; callers cannot grow a permanent cache through new contracts.
var dataValidators = struct {
	sync.Mutex
	items map[[32]byte]*list.Element
	lru   *list.List
	bytes int
}{items: map[[32]byte]*list.Element{}, lru: list.New()}

type cachedValidator struct {
	key    [32]byte
	size   int
	schema *jsonschema.Resolved
}

// ProtocolSchema returns an independent copy for adapters such as MCP that
// must advertise the shared wire schema instead of reflecting recursive types.
func ProtocolSchema(name string) (Schema, error) {
	raw, ok := protocolSchemas[name]
	if !ok {
		return nil, fmt.Errorf("unknown protocol schema %q", name)
	}
	var schema Schema
	err := json.Unmarshal([]byte(raw), &schema)
	return schema, err
}

func resolveSchema(schema Schema) (*jsonschema.Resolved, error) {
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256(raw)
	dataValidators.Lock()
	if element := dataValidators.items[key]; element != nil {
		dataValidators.lru.MoveToFront(element)
		cached := element.Value.(cachedValidator).schema
		dataValidators.Unlock()
		return cached, nil
	}
	dataValidators.Unlock()
	var s jsonschema.Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	resolved, err := s.Resolve(nil) // No remote loader; never fetch schema URLs.
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxJSONBytes {
		return resolved, nil
	}
	dataValidators.Lock()
	defer dataValidators.Unlock()
	if element := dataValidators.items[key]; element != nil {
		return element.Value.(cachedValidator).schema, nil
	}
	for len(dataValidators.items) >= 256 || dataValidators.bytes+len(raw) > 4<<20 {
		element := dataValidators.lru.Back()
		if element == nil {
			break
		}
		entry := element.Value.(cachedValidator)
		delete(dataValidators.items, entry.key)
		dataValidators.bytes -= entry.size
		dataValidators.lru.Remove(element)
	}
	dataValidators.items[key] = dataValidators.lru.PushFront(cachedValidator{key, len(raw), resolved})
	dataValidators.bytes += len(raw)
	return resolved, nil
}

func ValidateProtocol(name string, value any) error {
	if err := SafeJSON(value, 0); err != nil {
		return err
	}
	cached, ok := resolvedProtocols.Load(name)
	if !ok {
		raw, exists := protocolSchemas[name]
		if !exists {
			return fmt.Errorf("unknown protocol schema %q", name)
		}
		var schema Schema
		if err := json.Unmarshal([]byte(raw), &schema); err != nil {
			return err
		}
		compiled, err := resolveSchema(schema)
		if err != nil {
			return err
		}
		cached, _ = resolvedProtocols.LoadOrStore(name, compiled)
	}
	return cached.(*jsonschema.Resolved).Validate(value)
}

func ValidateData(schema Schema, value any) error {
	if err := SafeJSON(value, 0); err != nil {
		return err
	}
	compiled, err := resolveSchema(schema)
	if err != nil {
		return err
	}
	return compiled.Validate(value)
}

var schemaKeywords = strings.Fields("$schema $defs $ref type properties required additionalProperties items enum const minimum maximum exclusiveMinimum exclusiveMaximum minLength maxLength minItems maxItems title description format")

// ValidateSchema accepts a deliberately bounded 2020-12 profile. Structural
// schema edges and local references form a DAG; recursion is rejected.
func ValidateSchema(root Schema) error {
	if err := SafeJSON(root, 0); err != nil {
		return err
	}
	nodes := map[string]Schema{}
	var collect func(Schema, string) error
	collect = func(s Schema, path string) error {
		if len(nodes) >= 1024 {
			return fmt.Errorf("schema exceeds 1024 nodes")
		}
		nodes[path] = s
		for key := range s {
			if !slices.Contains(schemaKeywords, key) {
				return fmt.Errorf("%s/%s: unsupported schema keyword", path, key)
			}
		}
		if _, ok := s["$ref"]; ok {
			for key := range s {
				if !slices.Contains([]string{"$ref", "$defs", "$schema", "title", "description", "format"}, key) {
					return fmt.Errorf("%s: $ref supports annotation siblings only", path)
				}
			}
		}
		if draft, ok := s["$schema"]; ok && draft != "https://json-schema.org/draft/2020-12/schema" {
			return fmt.Errorf("%s: only JSON Schema 2020-12 is supported", path)
		}
		for _, key := range []string{"properties", "$defs"} {
			if raw, ok := s[key]; ok {
				children, ok := raw.(map[string]any)
				if !ok {
					return fmt.Errorf("%s/%s: expected object", path, key)
				}
				for name, rawChild := range children {
					child, ok := rawChild.(map[string]any)
					if !ok {
						return fmt.Errorf("%s/%s/%s: expected schema object", path, key, name)
					}
					if err := collect(child, path+"/"+key+"/"+escapePointer(name)); err != nil {
						return err
					}
				}
			}
		}
		for _, key := range []string{"items", "additionalProperties"} {
			if raw, ok := s[key]; ok {
				if child, ok := raw.(map[string]any); ok {
					if err := collect(child, path+"/"+key); err != nil {
						return err
					}
				} else if _, ok := raw.(bool); key != "additionalProperties" || !ok {
					return fmt.Errorf("%s/%s: expected schema%s", path, key, map[bool]string{true: " or boolean"}[key == "additionalProperties"])
				}
			}
		}
		return nil
	}
	if err := collect(root, "#"); err != nil {
		return err
	}
	active, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(path string) error {
		if active[path] {
			return fmt.Errorf("%s: recursive schema", path)
		}
		if done[path] {
			return nil
		}
		s, ok := nodes[path]
		if !ok {
			return fmt.Errorf("%s: unresolved local schema reference", path)
		}
		active[path] = true
		if raw, ok := s["$ref"]; ok {
			target, ok := raw.(string)
			if !ok || !strings.HasPrefix(target, "#/") {
				return fmt.Errorf("%s: only local JSON Pointer references are supported", path)
			}
			if err := visit(target); err != nil {
				return err
			}
		}
		for child := range nodes {
			if child != path && strings.HasPrefix(child, path+"/") {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		active[path] = false
		done[path] = true
		return nil
	}
	if err := visit("#"); err != nil {
		return err
	}
	expanded := deref(root, root)
	if expanded["type"] != "object" {
		return fmt.Errorf("schema root must declare object type")
	}
	if err := ValidateProtocol("data-schema", root); err != nil {
		return err
	}
	_, err := resolveSchema(root)
	return err
}

func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func PointerParts(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("invalid JSON Pointer %q", pointer)
	}
	parts := strings.Split(pointer[1:], "/")
	for i, part := range parts {
		for j := 0; j < len(part); j++ {
			if part[j] == '~' {
				if j+1 >= len(part) || (part[j+1] != '0' && part[j+1] != '1') {
					return nil, fmt.Errorf("invalid JSON Pointer escape")
				}
				j++
			}
		}
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

func deref(root, s Schema) Schema {
	for {
		reference, ok := s["$ref"].(string)
		if !ok {
			return s
		}
		parts, err := PointerParts(strings.TrimPrefix(reference, "#"))
		if err != nil {
			return nil
		}
		var node any = root
		for _, part := range parts {
			m, ok := node.(map[string]any)
			if !ok {
				return nil
			}
			node = m[part]
		}
		s, ok = node.(map[string]any)
		if !ok {
			return nil
		}
	}
}

func schemaTypes(s Schema) []string {
	if t, ok := s["type"].(string); ok {
		return []string{t}
	}
	var types []string
	if values, ok := s["type"].([]any); ok {
		for _, v := range values {
			if t, ok := v.(string); ok {
				types = append(types, t)
			}
		}
	}
	return types
}

func FieldSchema(root Schema, pointer string) (Schema, error) {
	parts, err := PointerParts(pointer)
	if err != nil {
		return nil, err
	}
	s := root
	for _, part := range parts {
		s = deref(root, s)
		if !slices.Contains(schemaTypes(s), "object") {
			return nil, fmt.Errorf("%s: field traversal requires objects", pointer)
		}
		properties, _ := s["properties"].(map[string]any)
		child, ok := properties[part].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: field is not explicitly declared", pointer)
		}
		s = child
	}
	return deref(root, s), nil
}

func scalarSchema(root Schema, pointer string) (Schema, error) {
	s, err := FieldSchema(root, pointer)
	if err != nil {
		return nil, err
	}
	types := schemaTypes(s)
	if len(types) == 0 {
		return nil, fmt.Errorf("%s: scalar type is required", pointer)
	}
	for _, t := range types {
		if !slices.Contains([]string{"string", "number", "integer", "boolean", "null"}, t) {
			return nil, fmt.Errorf("%s: not a scalar field", pointer)
		}
	}
	return s, nil
}

// ProjectSchema constructs an output schema without weakening the stored one.
// Parent/child overlapping selections are rejected rather than silently merged.
func ProjectSchema(root Schema, selection []string) (Schema, error) {
	if len(selection) == 0 {
		return root, nil
	}
	result := Schema{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
	if defs, ok := root["$defs"]; ok {
		result["$defs"] = defs
	}
	for i, pointer := range selection {
		for j, other := range selection {
			if i != j && (pointer == other || strings.HasPrefix(pointer, other+"/")) {
				return nil, fmt.Errorf("overlapping projection fields")
			}
		}
		if _, err := FieldSchema(root, pointer); err != nil {
			return nil, err
		}
		parts, _ := PointerParts(pointer)
		src, dst := deref(root, root), result
		for index, part := range parts {
			srcProps, _ := src["properties"].(map[string]any)
			child := deref(root, srcProps[part].(map[string]any))
			dstProps := dst["properties"].(map[string]any)
			if required, _ := src["required"].([]any); slices.Contains(required, any(part)) {
				old, _ := dst["required"].([]any)
				if !slices.Contains(old, any(part)) {
					dst["required"] = append(old, part)
				}
			}
			if index == len(parts)-1 {
				dstProps[part] = child
				break
			}
			next, ok := dstProps[part].(map[string]any)
			if !ok {
				next = Schema{"type": child["type"], "properties": map[string]any{}, "additionalProperties": false}
				dstProps[part] = next
			}
			src, dst = child, next
		}
	}
	return result, nil
}
