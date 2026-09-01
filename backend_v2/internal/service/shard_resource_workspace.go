package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strconv"

	"aladin/backend_v2/internal/shardv2"
)

type workspaceResourceProvider struct{ registry *EntityRegistry }

func (p *workspaceResourceProvider) ValidateResourceStage(ctx context.Context, definition shardv2.Resource) error {
	if definition.Source.Provider != "workspace.nodes" {
		return nil
	}
	view := ResourceView{Definition: definition, Params: definition.Source.Params, Query: shardv2.Query{Limit: shardv2.MaxLimit}}
	page, err := p.Snapshot(ctx, view)
	if err != nil {
		return err
	}
	ids := resourceNodeIDs(view.Params["ids"])
	unique := map[string]bool{}
	for _, id := range ids {
		unique[id] = true
	}
	if len(page.Records) != len(unique) {
		return ResourceFailure("not-found", "A declared workspace source is missing")
	}
	for _, record := range page.Records {
		value, err := shardv2.DecodeJSON(record.Data)
		if err != nil || shardv2.ValidateData(definition.Schema, value) != nil {
			return ResourceFailure("invalid-schema", "Workspace response does not match the declared schema")
		}
	}
	return nil
}

func NewWorkspaceResourceProvider(registry *EntityRegistry) ResourceProvider {
	return &workspaceResourceProvider{registry: registry}
}
func (p *workspaceResourceProvider) Profile() shardv2.ProviderProfile {
	return shardv2.ProviderProfile{Version: 1, Operations: []string{"snapshot"}, Observation: "refresh-snapshots", ParamsSchema: shardv2.Schema{"type": "object", "properties": map[string]any{"ids": map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 256}, "minItems": 1, "maxItems": 500}}, "required": []any{"ids"}, "additionalProperties": false}}
}
func resourceNodeIDs(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		id, ok := item.(string)
		if !ok {
			return nil
		}
		out = append(out, id)
	}
	return out
}
func (p *workspaceResourceProvider) Authorize(ctx context.Context, view ResourceView) error {
	if p.registry == nil {
		return ResourceFailure("source-unavailable", "Workspace registry unavailable")
	}
	if view.Definition.Kind != "collection" {
		return ResourceFailure("unsupported-capability", "Workspace nodes are a collection")
	}
	granted := resourceNodeIDs(view.Definition.Source.Params["ids"])
	requested := resourceNodeIDs(view.Params["ids"])
	if len(granted) == 0 || len(requested) == 0 {
		return ResourceFailure("forbidden", "Workspace IDs must have a fixed release grant")
	}
	if view.ID != "" && !slices.Contains(requested, view.ID) {
		return ResourceFailure("forbidden", "Workspace ID is outside the requested view")
	}
	for _, id := range requested {
		if !slices.Contains(granted, id) {
			return ResourceFailure("forbidden", "Workspace ID is outside the release grant")
		}
		_, _, reader, err := p.registry.Resolve(id)
		if err != nil {
			return ResourceFailure("bad-request", "Unsupported workspace entity")
		}
		if view.Definition.Observe != nil && !reader.Observable() {
			return ResourceFailure("unsupported-capability", "Workspace entity cannot be observed")
		}
	}
	if view.Query.Where != nil || len(view.Query.OrderBy) > 0 || view.Query.Cursor != nil {
		return ResourceFailure("unsupported-capability", "Workspace source does not support structured queries or cursors")
	}
	// Never silently truncate an explicit list of granted/requested nodes.
	if view.ID == "" && len(requested) > view.Query.Limit {
		return ResourceFailure("bad-request", "Workspace IDs exceed the requested view limit")
	}
	return RequireScope(ctx, ScopeArtifactsRead)
}
func (p *workspaceResourceProvider) Snapshot(ctx context.Context, view ResourceView) (ResourcePage, error) {
	if err := p.Authorize(ctx, view); err != nil {
		return ResourcePage{}, err
	}
	records := []shardv2.Record{}
	ids := resourceNodeIDs(view.Params["ids"])
	sort.Strings(ids)
	ids = slices.Compact(ids)
	for _, ref := range ids {
		if view.ID != "" && view.ID != ref {
			continue
		}
		_, id, reader, err := p.registry.Resolve(ref)
		if err != nil {
			return ResourcePage{}, err
		}
		node, err := reader.NodeView(ctx, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return ResourcePage{}, err
		}
		// Version is an opaque string; schemaVersion refers to the declared resource
		// envelope, not a claim about the external service's database schema.
		node.ID = ref
		data, _ := json.Marshal(node)
		records = append(records, shardv2.Record{ID: ref, Revision: strconv.FormatUint(node.Seq, 10), SchemaVersion: view.Definition.SchemaVersion, Data: data})
	}
	return ResourcePage{Records: records}, nil
}
func (*workspaceResourceProvider) Mutate(context.Context, ResourceView, shardv2.Command) (ResourceMutationResult, error) {
	return ResourceMutationResult{}, ResourceFailure("unsupported-capability", "Workspace nodes are read-only")
}
