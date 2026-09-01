package shardv2

import "fmt"

func PointerValue(value any, pointer string) (any, bool) {
	parts, err := PointerParts(pointer)
	if err != nil {
		return nil, false
	}
	for _, part := range parts {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return value, true
}

// ProjectData runs on the server BEFORE data reaches any transport. Field paths
// are compiler-validated object paths; nullable ancestors retain explicit null.
func ProjectData(value any, selection []string) (any, error) {
	if len(selection) == 0 {
		return value, nil
	}
	source, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("projection requires an object")
	}
	output := map[string]any{}
	for _, pointer := range selection {
		parts, err := PointerParts(pointer)
		if err != nil || len(parts) == 0 {
			return nil, fmt.Errorf("invalid projection path")
		}
		src, dst := source, output
		for i, part := range parts {
			child, exists := src[part]
			if !exists {
				break
			}
			if i == len(parts)-1 || child == nil {
				dst[part] = child
				break
			}
			next, ok := child.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("non-object projection ancestor")
			}
			if _, exists := dst[part]; !exists {
				dst[part] = map[string]any{}
			}
			target, ok := dst[part].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("overlapping projection")
			}
			src, dst = next, target
		}
	}
	return output, nil
}
