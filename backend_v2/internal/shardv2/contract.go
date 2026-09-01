package shardv2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

func Compile(data []byte, providers Registry) (*Compiled, error) {
	value, err := DecodeJSON(data)
	if err != nil {
		return nil, err
	}
	if err := ValidateProtocol("contract", value); err != nil {
		return nil, err
	}
	var c Contract
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	datasets := map[string]bool{}
	for id, r := range c.Resources {
		at := func(err error) (*Compiled, error) { return nil, fmt.Errorf("resources.%s: %w", id, err) }
		if r.URI != "shard://self/resources/"+id {
			return at(fmt.Errorf("URI must match resource ID"))
		}
		if err := ValidateSchema(r.Schema); err != nil {
			return at(err)
		}
		p, ok := providers[r.Source.Provider]
		if !ok || (r.Source.Version != 0 && r.Source.Version != p.Version) {
			return at(fmt.Errorf("unknown provider/version"))
		}
		if !slices.Contains(r.Operations, "snapshot") {
			return at(fmt.Errorf("snapshot capability is required"))
		}
		for _, op := range r.Operations {
			if !slices.Contains(p.Operations, op) {
				return at(fmt.Errorf("unsupported capability %s", op))
			}
		}
		if r.Observe != nil && p.Observation == "" {
			return at(fmt.Errorf("provider cannot observe"))
		}
		if p.Owned {
			if r.Source.Dataset == "" {
				return at(fmt.Errorf("owned resource requires dataset"))
			}
			if datasets[r.Source.Dataset] {
				return at(fmt.Errorf("dataset already declared; aliases are not supported"))
			}
			datasets[r.Source.Dataset] = true
		} else if r.Source.Dataset != "" {
			return at(fmt.Errorf("external provider cannot select storage dataset"))
		}
		declared := append([]string{}, r.Operations...)
		if r.Observe != nil {
			declared = append(declared, "observe")
		}
		for _, caps := range [][]string{r.Exposure.App, r.Exposure.Agent} {
			for _, cap := range caps {
				if !slices.Contains(declared, cap) {
					return at(fmt.Errorf("exposure exceeds declared capabilities: %s", cap))
				}
			}
		}
		for _, pointer := range append(append([]string{}, r.Query.FilterFields...), r.Query.SortFields...) {
			if _, err := scalarSchema(r.Schema, pointer); err != nil {
				return at(err)
			}
		}
		if len(r.Query.FilterFields)+len(r.Query.SortFields) > 0 && !slices.Contains(r.Operations, "query") {
			return at(fmt.Errorf("query fields require query capability"))
		}
		if err := validateParams(p.ParamsSchema, r.Source.Params, nil, false); err != nil {
			return at(err)
		}
	}
	validateHandler := func(kind, id string, handler RuntimeHandler) error {
		for _, capability := range handler.Capabilities {
			parts := strings.SplitN(capability, ":", 2)
			binding, ok := c.Bindings[parts[0]]
			resource := c.Resources[binding.Resource]
			if !ok || len(parts) != 2 || !slices.Contains(resource.Operations, parts[1]) {
				return fmt.Errorf("%s.%s: capability %s is not declared by a binding", kind, id, capability)
			}
		}
		return nil
	}
	if c.GraphQL != nil {
		for id, handler := range c.GraphQL.Resolvers {
			if err := validateHandler("graphql.resolvers", id, handler); err != nil {
				return nil, err
			}
		}
		for id, operation := range c.GraphQL.Operations {
			document := strings.TrimSpace(operation.Document)
			if !strings.HasPrefix(document, "query ") && !strings.HasPrefix(document, "mutation ") {
				return nil, fmt.Errorf("graphql.operations.%s: document must be a named query or mutation", id)
			}
		}
	}
	for id, lambda := range c.Lambdas {
		if err := validateHandler("lambdas", id, lambda.RuntimeHandler); err != nil {
			return nil, err
		}
	}
	out := &Compiled{Contract: c, Source: append([]byte(nil), data...), OutputSchemas: map[string]Schema{}}
	dependencies := map[string][]string{}
	for id, b := range c.Bindings {
		at := func(err error) (*Compiled, error) { return nil, fmt.Errorf("bindings.%s: %w", id, err) }
		r, ok := c.Resources[b.Resource]
		if !ok {
			return at(fmt.Errorf("unknown resource"))
		}
		if b.InputsSchema != nil {
			if err := ValidateSchema(b.InputsSchema); err != nil {
				return at(err)
			}
		}
		if b.Query != nil {
			if err := ValidateQuery(r, *b.Query); err != nil {
				return at(err)
			}
		}
		output, err := ProjectSchema(r.Schema, b.Select)
		if err != nil {
			return at(err)
		}
		out.OutputSchemas[id] = output
		params := map[string]any{}
		for k, v := range r.Source.Params {
			params[k] = v
		}
		dynamic := map[string]bool{}
		for name, v := range b.Params {
			expr, isObject := v.(map[string]any)
			if isObject {
				if literal, ok := expr["literal"]; ok {
					params[name] = literal
					continue
				}
				if input, ok := expr["input"].(string); ok {
					if _, err := FieldSchema(b.InputsSchema, "/"+escapePointer(input)); err != nil {
						return at(fmt.Errorf("unknown input %s", input))
					}
					dynamic[name] = true
					delete(params, name)
					continue
				}
				if dependency, ok := expr["binding"].(string); ok {
					dep, exists := c.Bindings[dependency]
					if !exists {
						return at(fmt.Errorf("unknown dependency %s", dependency))
					}
					depResource := c.Resources[dep.Resource]
					if depResource.Kind != "singleton" {
						return at(fmt.Errorf("binding dependency must be singleton"))
					}
					pointer := expr["pointer"].(string)
					depOutput, err := ProjectSchema(depResource.Schema, dep.Select)
					if err != nil {
						return at(err)
					}
					if _, err := FieldSchema(depOutput, strings.TrimPrefix(pointer, "/data")); err != nil {
						return at(err)
					}
					dependencies[id] = append(dependencies[id], dependency)
					dynamic[name] = true
					delete(params, name)
					continue
				}
			}
			params[name] = v
		}
		if err := validateParams(providers[r.Source.Provider].ParamsSchema, params, dynamic, true); err != nil {
			return at(err)
		}
	}
	active, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if active[id] {
			return fmt.Errorf("binding dependency cycle at %s", id)
		}
		if done[id] {
			return nil
		}
		active[id] = true
		sort.Strings(dependencies[id])
		for _, dep := range dependencies[id] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		active[id] = false
		done[id] = true
		out.BindingOrder = append(out.BindingOrder, id)
		return nil
	}
	ids := make([]string, 0, len(c.Bindings))
	for id := range c.Bindings {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	hash := sha256.Sum256(data)
	out.Hash = hex.EncodeToString(hash[:])
	return out, nil
}

func validateParams(schema Schema, params map[string]any, dynamic map[string]bool, complete bool) error {
	if schema == nil {
		schema = Schema{"type": "object", "additionalProperties": false}
	}
	// Copy the schema; required dynamic values are checked after binding resolution.
	raw, _ := json.Marshal(schema)
	var partial Schema
	_ = json.Unmarshal(raw, &partial)
	properties, _ := partial["properties"].(map[string]any)
	for field := range dynamic {
		if _, ok := properties[field]; !ok {
			return fmt.Errorf("unknown provider parameter %s", field)
		}
	}
	required, _ := partial["required"].([]any)
	var filtered []any
	if complete {
		for _, name := range required {
			if !dynamic[name.(string)] {
				filtered = append(filtered, name)
			}
		}
	}
	if len(filtered) > 0 {
		partial["required"] = filtered
	} else {
		delete(partial, "required")
	}
	if params == nil {
		params = map[string]any{}
	}
	return ValidateData(partial, params)
}

// EffectiveCapabilities never grants access: callers must supply current,
// server-derived allowed capabilities after authorization.
func EffectiveCapabilities(r Resource, audience string, allowed []string) []string {
	var exposure []string
	switch audience {
	case "app":
		exposure = r.Exposure.App
	case "agent":
		exposure = r.Exposure.Agent
	default:
		return nil
	}
	var out []string
	for _, cap := range exposure {
		if slices.Contains(allowed, cap) {
			out = append(out, cap)
		}
	}
	return out
}

func NormalizeQuery(q Query) (Query, error) {
	raw, err := json.Marshal(q)
	if err != nil {
		return Query{}, err
	}
	v, err := DecodeJSON(raw)
	if err != nil {
		return Query{}, err
	}
	if err := ValidateProtocol("query", v); err != nil {
		return Query{}, err
	}
	var normalized Query
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return Query{}, err
	}
	return normalized, nil
}

func ValidateQuery(r Resource, q Query) error {
	// Normalize typed Go callers (for example []string and int operands) to the
	// same JSON value domain used by decoded bridge requests before traversal.
	normalized, err := NormalizeQuery(q)
	if err != nil {
		return err
	}
	q = normalized
	limit := q.Limit
	if limit == 0 {
		limit = DefaultLimit
	}
	max := r.Query.MaxLimit
	if max == 0 {
		max = MaxLimit
	}
	if limit > max {
		return fmt.Errorf("query exceeds resource limit")
	}
	if (q.Where != nil || len(q.OrderBy) > 0) && !slices.Contains(r.Operations, "query") {
		return fmt.Errorf("unsupported query capability")
	}
	seen := map[string]bool{}
	for _, order := range q.OrderBy {
		if seen[order.Field] || !slices.Contains(r.Query.SortFields, order.Field) {
			return fmt.Errorf("invalid sort field %s", order.Field)
		}
		seen[order.Field] = true
		if _, err := scalarSchema(r.Schema, order.Field); err != nil {
			return err
		}
	}
	count := 0
	var walk func(Predicate, int) error
	walk = func(p Predicate, depth int) error {
		if depth > 8 {
			return fmt.Errorf("query depth exceeds 8")
		}
		if len(p.And) > 0 || len(p.Or) > 0 {
			for _, child := range append(p.And, p.Or...) {
				if err := walk(child, depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		count++
		if count > 32 {
			return fmt.Errorf("query exceeds 32 predicates")
		}
		if !slices.Contains(r.Query.FilterFields, p.Field) {
			return fmt.Errorf("invalid filter field %s", p.Field)
		}
		s, err := scalarSchema(r.Schema, p.Field)
		if err != nil {
			return err
		}
		if p.Op == "exists" {
			return nil
		}
		values := []any{p.Value}
		if p.Op == "in" {
			values = p.Value.([]any)
		}
		for _, value := range values {
			if slices.Contains([]string{"gt", "gte", "lt", "lte"}, p.Op) {
				if _, ok := value.(float64); !ok {
					return fmt.Errorf("range operand must be numeric")
				}
				for _, t := range schemaTypes(s) {
					if t != "number" && t != "integer" && t != "null" {
						return fmt.Errorf("range field must be numeric")
					}
				}
			}
			if err := ValidateData(s, value); err != nil {
				return err
			}
		}
		return nil
	}
	if q.Where != nil {
		return walk(*q.Where, 1)
	}
	return nil
}

func ValidateEvent(data []byte, r Resource, output Schema) (Event, error) {
	v, err := DecodeJSON(data)
	if err != nil {
		return Event{}, err
	}
	if err := ValidateProtocol("event", v); err != nil {
		return Event{}, err
	}
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return Event{}, err
	}
	records := event.Records
	if event.Record != nil {
		records = []Record{*event.Record}
	}
	if r.Kind == "singleton" && len(records) > 1 {
		return Event{}, fmt.Errorf("singleton has multiple records")
	}
	seen := map[string]bool{}
	for _, record := range records {
		if seen[record.ID] {
			return Event{}, fmt.Errorf("duplicate snapshot record")
		}
		seen[record.ID] = true
		if r.Kind == "singleton" && record.ID != "value" {
			return Event{}, fmt.Errorf("invalid singleton ID")
		}
		if record.SchemaVersion != r.SchemaVersion {
			return Event{}, fmt.Errorf("schema version mismatch")
		}
		if len(record.Data) > MaxRecordBytes {
			return Event{}, fmt.Errorf("record exceeds byte limit")
		}
		value, err := DecodeJSON(record.Data)
		if err != nil {
			return Event{}, err
		}
		if err := ValidateData(output, value); err != nil {
			return Event{}, err
		}
	}
	if r.Kind == "singleton" && event.Op == "delete" && event.ID != "value" {
		return Event{}, fmt.Errorf("invalid singleton ID")
	}
	return event, nil
}
