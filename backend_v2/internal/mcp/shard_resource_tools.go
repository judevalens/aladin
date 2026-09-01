package mcpserver

import (
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardv2"
	"context"
	"encoding/json"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"net/url"
	"strings"
	"time"
)

type shardResourceTools struct {
	resources service.ShardResourceService
	catalog   service.ShardCatalogService
	graphql   service.ShardGraphQLService
}

func readOnlyTool(title string) *sdkmcp.ToolAnnotations {
	closed := false
	return &sdkmcp.ToolAnnotations{Title: title, ReadOnlyHint: true, OpenWorldHint: &closed}
}
func resourceMCPInputSchema(queryRequired bool) map[string]any {
	query, _ := shardv2.ProtocolSchema("query")
	definitions := query["$defs"]
	delete(query, "$defs")
	delete(query, "$schema")
	token := map[string]any{"type": "string", "minLength": 1, "maxLength": 256}
	required := []string{"uri"}
	if queryRequired {
		required = append(required, "query")
	}
	return map[string]any{"type": "object", "additionalProperties": false, "required": required, "$defs": definitions, "properties": map[string]any{"uri": map[string]any{"type": "string", "minLength": 1, "maxLength": 1024}, "contractHash": token, "id": token, "query": query}}
}

// RawMessage is JSON on the wire, but generic Go schema inference sees []byte.
// Reuse the protocol schemas so SDK output validation agrees with both clients.
func resourceMCPOutputSchema(mutation bool) map[string]any {
	token := map[string]any{"type": "string", "minLength": 1, "maxLength": 256}
	if mutation {
		record, _ := shardv2.ProtocolSchema("record")
		return map[string]any{"type": "object", "additionalProperties": false, "required": []string{"requestId"}, "properties": map[string]any{"requestId": token, "record": record, "tombstone": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"id", "revision"}, "properties": map[string]any{"id": token, "revision": token}}}}
	}
	descriptor, _ := shardv2.ProtocolSchema("descriptor")
	snapshot, _ := shardv2.ProtocolSchema("snapshot")
	return map[string]any{"type": "object", "additionalProperties": false, "required": []string{"uri", "contractHash", "environment", "descriptor"}, "properties": map[string]any{"uri": map[string]any{"type": "string"}, "contractHash": token, "environment": map[string]any{"const": "published"}, "descriptor": descriptor, "snapshot": snapshot}}
}

type findShardResourcesInput struct {
	Query string `json:"query,omitempty"`
	Limit int    `json:"limit,omitempty"`
}
type findShardResourcesOutput struct {
	Resources []service.ShardCatalogEntry `json:"resources"`
}
type shardResourceInput struct {
	URI          string         `json:"uri"`
	ContractHash string         `json:"contractHash,omitempty"`
	ID           string         `json:"id,omitempty"`
	Query        *shardv2.Query `json:"query,omitempty"`
}
type shardResourceOutput struct {
	URI          string                     `json:"uri"`
	ContractHash string                     `json:"contractHash"`
	Environment  string                     `json:"environment"`
	Descriptor   service.ResourceDescriptor `json:"descriptor"`
	Snapshot     *service.ResourceSnapshot  `json:"snapshot,omitempty"`
}
type mutateShardResourceInput struct {
	URI          string         `json:"uri"`
	ContractHash string         `json:"contractHash"`
	Op           string         `json:"op"`
	ID           string         `json:"id,omitempty"`
	RequestID    string         `json:"requestId"`
	BaseRevision string         `json:"baseRevision,omitempty"`
	Data         map[string]any `json:"data,omitempty"`
}
type executeShardOperationInput struct {
	ShardID      string         `json:"shardId"`
	OperationID  string         `json:"operationId"`
	ContractHash string         `json:"contractHash,omitempty"`
	Variables    map[string]any `json:"variables,omitempty"`
}

func registerShardResourceTools(server *sdkmcp.Server, resources service.ShardResourceService, catalog service.ShardCatalogService, graphql service.ShardGraphQLService) {
	if resources == nil || catalog == nil {
		return
	}
	t := shardResourceTools{resources: resources, catalog: catalog, graphql: graphql}
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "find_shard_resources", Description: "Find published shard resources, their schemas and current agent capabilities. Reads protected release metadata; no open iframe required.", Annotations: readOnlyTool("Find shard resources")}, t.find)
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "describe_shard_resource", OutputSchema: resourceMCPOutputSchema(false), InputSchema: resourceMCPInputSchema(false), Description: "Describe a published shard:// resource and its current agent capabilities and contract hash.", Annotations: readOnlyTool("Describe shard resource")}, t.describe)
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "read_shard_resource", OutputSchema: resourceMCPOutputSchema(false), InputSchema: resourceMCPInputSchema(false), Description: "Read current canonical published resource records while its UI may be closed. Optional id selects one record. Revisions are opaque strings.", Annotations: readOnlyTool("Read shard resource")}, t.read)
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "query_shard_resource", OutputSchema: resourceMCPOutputSchema(false), InputSchema: resourceMCPInputSchema(true), Description: "Query declared scalar fields with bounded filter/order/limit/cursor. Backend enforces declared query capabilities; pages are read-current.", Annotations: readOnlyTool("Query shard resource")}, t.read)
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "mutate_shard_resource", OutputSchema: resourceMCPOutputSchema(true), Description: "Insert, fully replace (update), or delete a published shard resource record. Requires granted agent write capability, current contractHash and requestId; update/delete require baseRevision. Retain the same requestId and exact payload when retrying an unknown outcome within 24 hours.", Annotations: destructiveTool("Mutate shard resource")}, t.mutate)
	if graphql != nil && graphql.Enabled() {
		sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "execute_shard_operation", Description: "Execute a named, published GraphQL query or mutation declared by a shard. The backend pins the active release, checks agent exposure, and does not accept raw GraphQL text."}, t.executeOperation)
	}
}
func (t shardResourceTools) executeOperation(ctx context.Context, _ *sdkmcp.CallToolRequest, in executeShardOperationInput) (*sdkmcp.CallToolResult, map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	target := service.ResourceTarget{ShardID: in.ShardID, Environment: service.ChannelPublished, Audience: "agent", ContractHash: in.ContractHash}
	if target.ContractHash == "" {
		hello, err := t.resources.Hello(ctx, target)
		if err != nil {
			return nil, nil, err
		}
		target.ContractHash, _ = hello["contractHash"].(string)
	}
	raw, err := t.graphql.Execute(ctx, target, service.ShardGraphQLRequest{OperationID: in.OperationID, Variables: in.Variables})
	if err != nil {
		return nil, nil, err
	}
	value, err := shardv2.DecodeJSON(raw)
	if err != nil {
		return nil, nil, err
	}
	return nil, value.(map[string]any), nil
}
func (t shardResourceTools) find(ctx context.Context, _ *sdkmcp.CallToolRequest, in findShardResourcesInput) (*sdkmcp.CallToolResult, findShardResourcesOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	entries, err := t.catalog.Find(ctx, in.Query, in.Limit)
	return nil, findShardResourcesOutput{Resources: entries}, err
}
func resourceURITarget(uri, hash string) (service.ResourceTarget, service.ResourceRequest, error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "shard" || u.Host == "" || u.Host == "self" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Port() != "" {
		return service.ResourceTarget{}, service.ResourceRequest{}, service.ResourceFailure("bad-request", "Expected shard://<shard-id>/resources/<resource-id>")
	}
	parts := strings.Split(u.Path, "/")
	if len(parts) != 3 || parts[1] != "resources" || parts[2] == "" {
		return service.ResourceTarget{}, service.ResourceRequest{}, service.ResourceFailure("bad-request", "Invalid resource URI")
	}
	return service.ResourceTarget{ShardID: u.Host, Environment: service.ChannelPublished, Audience: "agent", ContractHash: hash}, service.ResourceRequest{Resource: parts[2]}, nil
}
func (t shardResourceTools) resolve(ctx context.Context, in shardResourceInput) (service.ResourceTarget, service.ResourceRequest, shardResourceOutput, error) {
	target, request, err := resourceURITarget(in.URI, in.ContractHash)
	out := shardResourceOutput{URI: in.URI, Environment: "published"}
	if err != nil {
		return target, request, out, err
	}
	if target.ContractHash == "" {
		hello, err := t.resources.Hello(ctx, target)
		if err != nil {
			return target, request, out, err
		}
		target.ContractHash, _ = hello["contractHash"].(string)
	}
	descriptor, err := t.resources.Describe(ctx, target, request)
	out.ContractHash = target.ContractHash
	out.Descriptor = descriptor
	request.ID, request.Query = in.ID, in.Query
	return target, request, out, err
}
func (t shardResourceTools) describe(ctx context.Context, _ *sdkmcp.CallToolRequest, in shardResourceInput) (*sdkmcp.CallToolResult, shardResourceOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, _, out, err := t.resolve(ctx, in)
	return nil, out, err
}
func (t shardResourceTools) read(ctx context.Context, _ *sdkmcp.CallToolRequest, in shardResourceInput) (*sdkmcp.CallToolResult, shardResourceOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	target, request, out, err := t.resolve(ctx, in)
	if err != nil {
		return nil, out, err
	}
	snapshot, err := t.resources.Read(ctx, target, request)
	if err == nil {
		out.Snapshot = &snapshot
	}
	return nil, out, err
}
func (t shardResourceTools) mutate(ctx context.Context, _ *sdkmcp.CallToolRequest, in mutateShardResourceInput) (*sdkmcp.CallToolResult, service.ResourceMutationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	target, request, err := resourceURITarget(in.URI, in.ContractHash)
	if err != nil {
		return nil, service.ResourceMutationResult{}, err
	}
	request.ID = in.ID
	var data json.RawMessage
	if in.Data != nil {
		data, err = json.Marshal(in.Data)
		if err != nil {
			return nil, service.ResourceMutationResult{}, err
		}
	}
	result, err := t.resources.Mutate(ctx, target, service.ResourceMutation{ResourceRequest: request, Op: in.Op, RequestID: in.RequestID, BaseRevision: in.BaseRevision, Data: data})
	return nil, result, err
}
