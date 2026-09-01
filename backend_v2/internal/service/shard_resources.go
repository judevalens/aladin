package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"aladin/backend_v2/internal/shardv2"
)

type shardResourceService struct {
	artifacts      ArtifactService
	releases       ResourceReleaseReader
	providers      map[string]ResourceProvider
	profiles       shardv2.Registry
	options        ResourceServiceOptions
	mu             sync.Mutex
	subscriptions  map[string]int
	requestBudgets map[string]resourceRequestBudget
}
type resourceRequestBudget struct {
	tokens  float64
	updated time.Time
}

func NewShardResourceService(artifacts ArtifactService, releases ResourceReleaseReader, providers map[string]ResourceProvider, options ResourceServiceOptions) ShardResourceService {
	if options.RefreshInterval <= 0 {
		options.RefreshInterval = time.Second
	}
	if options.MaxSubscriptionsPerPrincipal <= 0 {
		options.MaxSubscriptionsPerPrincipal = 128
	}
	if options.RequestsPerSecond <= 0 {
		options.RequestsPerSecond = 64
	}
	if options.RequestBurst <= 0 {
		options.RequestBurst = 256
	}
	s := &shardResourceService{artifacts: artifacts, releases: releases, providers: map[string]ResourceProvider{}, profiles: shardv2.Registry{}, options: options, subscriptions: map[string]int{}}
	s.requestBudgets = map[string]resourceRequestBudget{}
	for name, provider := range providers {
		s.providers[name] = provider
		s.profiles[name] = provider.Profile()
	}
	return s
}

func (s *shardResourceService) admit(ctx context.Context) error {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	if p.ActorType == ActorTypeContentToken {
		return ErrForbidden
	}
	now, key := time.Now(), resourcePrincipalKey(p)
	s.mu.Lock()
	defer s.mu.Unlock()
	budget, exists := s.requestBudgets[key]
	if !exists {
		if len(s.requestBudgets) >= 1024 {
			for actor, entry := range s.requestBudgets {
				if now.Sub(entry.updated) > time.Minute {
					delete(s.requestBudgets, actor)
				}
			}
		}
		if len(s.requestBudgets) >= 1024 {
			return ResourceFailure("rate-limited", "Resource admission is busy")
		}
		budget = resourceRequestBudget{tokens: float64(s.options.RequestBurst), updated: now}
	}
	budget.tokens = min(float64(s.options.RequestBurst), budget.tokens+now.Sub(budget.updated).Seconds()*float64(s.options.RequestsPerSecond))
	budget.updated = now
	if budget.tokens < 1 {
		s.requestBudgets[key] = budget
		return ResourceFailure("rate-limited", "Resource request budget exceeded")
	}
	budget.tokens--
	s.requestBudgets[key] = budget
	return nil
}

func resourceHash(value any) string {
	raw, _ := json.Marshal(value)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
func resourcePrincipalKey(p Principal) string {
	return resourceHash([]string{p.UserID, p.ActorType, p.ActorID})
}

func (s *shardResourceService) resolveRelease(ctx context.Context, target ResourceTarget, requireHash bool) (Principal, ResourceRelease, *shardv2.Compiled, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return Principal{}, ResourceRelease{}, nil, err
	}
	if principal.ActorType == ActorTypeContentToken || (target.Audience != "app" && target.Audience != "agent") {
		return principal, ResourceRelease{}, nil, ErrForbidden
	}
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return principal, ResourceRelease{}, nil, err
	}
	if target.Environment != ChannelDraft && target.Environment != ChannelPublished {
		return principal, ResourceRelease{}, nil, ResourceFailure("bad-request", "Invalid resource environment")
	}
	rec, err := s.artifacts.Get(ctx, target.ShardID)
	if err != nil {
		return principal, ResourceRelease{}, nil, err
	}
	if rec.Type != "app" {
		return principal, ResourceRelease{}, nil, ErrNotFound
	}
	release, err := s.releases.ActiveResourceRelease(ctx, principal.UserID, target.ShardID, target.Environment)
	if err != nil {
		return principal, release, nil, err
	}
	if requireHash && (target.ContractHash == "" || release.Hash != target.ContractHash) {
		return principal, release, nil, ResourceFailure("contract-changed", "Reload the shard's active release")
	}
	compiled, err := shardv2.Compile(release.Source, s.profiles)
	if err != nil || compiled.Hash != release.Hash || release.Generation == "" || release.BuildID == "" {
		return principal, release, nil, ResourceFailure("invalid-schema", "Active resource release is invalid")
	}
	return principal, release, compiled, nil
}

func (s *shardResourceService) Hello(ctx context.Context, target ResourceTarget) (map[string]any, error) {
	if err := s.admit(ctx); err != nil {
		return nil, err
	}
	_, release, compiled, err := s.resolveRelease(ctx, target, false)
	if err != nil {
		return nil, err
	}
	return map[string]any{"protocol": shardv2.BridgeVersion, "streamProtocol": shardv2.StreamVersion, "contractHash": release.Hash, "buildId": release.BuildID,
		"methods":  []string{"hello", "theme.get", "resource.describe", "resource.read", "resource.query", "resource.subscribe", "resource.unsubscribe", "resource.insert", "resource.update", "resource.delete", "graphql.execute", "lambda.invoke"},
		"bindings": compiled.Contract.Bindings, "limits": map[string]int{"recordBytes": shardv2.MaxRecordBytes, "viewRecords": shardv2.MaxLimit, "subscriptions": 32}}, nil
}

func (s *shardResourceService) resolve(ctx context.Context, target ResourceTarget, request ResourceRequest, capability string) (ResourceView, ResourceDescriptor, ResourceProvider, error) {
	principal, release, compiled, err := s.resolveRelease(ctx, target, true)
	if err != nil {
		return ResourceView{}, ResourceDescriptor{}, nil, err
	}
	return s.resolveView(ctx, target, request, capability, principal, release, compiled, map[string]bool{})
}

func (s *shardResourceService) resolveView(ctx context.Context, target ResourceTarget, request ResourceRequest, capability string, principal Principal, release ResourceRelease, compiled *shardv2.Compiled, visiting map[string]bool) (view ResourceView, descriptor ResourceDescriptor, provider ResourceProvider, err error) {
	fail := func(code, message string) (ResourceView, ResourceDescriptor, ResourceProvider, error) {
		return view, descriptor, provider, ResourceFailure(code, message)
	}
	if (request.Binding == "") == (request.Resource == "") {
		return fail("bad-request", "Specify exactly one binding or resource")
	}
	binding := shardv2.Binding{Resource: request.Resource}
	if request.Binding != "" {
		var ok bool
		binding, ok = compiled.Contract.Bindings[request.Binding]
		if !ok {
			return fail("not-found", "Unknown resource binding")
		}
		if visiting[request.Binding] {
			return fail("invalid-schema", "Binding dependency cycle")
		}
		visiting[request.Binding] = true
		defer delete(visiting, request.Binding)
	}
	definition, ok := compiled.Contract.Resources[binding.Resource]
	if !ok {
		return fail("not-found", "Unknown resource")
	}
	provider = s.providers[definition.Source.Provider]
	allowed := []string{"snapshot", "query", "observe"}
	if HasScope(ctx, ScopeArtifactsWrite) && (target.Environment != ChannelDraft || provider.Profile().Owned) {
		allowed = append(allowed, "insert", "update", "delete")
	}
	caps := shardv2.EffectiveCapabilities(definition, target.Audience, allowed)
	if !slices.Contains(caps, capability) {
		return fail("forbidden", "Resource capability is not granted")
	}
	inputs := request.Inputs
	if inputs == nil {
		inputs = map[string]any{}
	}
	inputsSchema := binding.InputsSchema
	if inputsSchema == nil {
		inputsSchema = shardv2.Schema{"type": "object", "additionalProperties": false}
	}
	if err := shardv2.ValidateData(inputsSchema, inputs); err != nil {
		return fail("bad-request", "Invalid binding inputs")
	}
	params := map[string]any{}
	for name, value := range definition.Source.Params {
		params[name] = value
	}
	for name, value := range binding.Params {
		expression, isObject := value.(map[string]any)
		if !isObject {
			params[name] = value
			continue
		}
		if literal, ok := expression["literal"]; ok {
			params[name] = literal
			continue
		}
		if input, ok := expression["input"].(string); ok {
			value, exists := inputs[input]
			if !exists {
				return fail("bad-request", "Required binding input is missing")
			}
			params[name] = value
			continue
		}
		if dependency, ok := expression["binding"].(string); ok {
			dependencyView, _, dependencyProvider, dependencyErr := s.resolveView(ctx, target, ResourceRequest{Binding: dependency}, "snapshot", principal, release, compiled, visiting)
			if dependencyErr != nil {
				return view, descriptor, provider, dependencyErr
			}
			page, dependencyErr := s.snapshot(ctx, dependencyView, dependencyProvider)
			if dependencyErr != nil {
				return view, descriptor, provider, dependencyErr
			}
			if len(page.Records) != 1 {
				return fail("source-unavailable", "Binding dependency has no value yet")
			}
			var data any
			if json.Unmarshal(page.Records[0].Data, &data) != nil {
				return fail("invalid-schema", "Invalid dependency data")
			}
			value, found := shardv2.PointerValue(map[string]any{"data": data}, expression["pointer"].(string))
			if !found {
				return fail("source-unavailable", "Binding dependency pointer is missing")
			}
			params[name] = value
			continue
		}
		params[name] = value
	}
	if err := shardv2.ValidateData(provider.Profile().ParamsSchema, params); err != nil {
		return fail("bad-request", "Invalid source parameters")
	}
	query := shardv2.Query{}
	if binding.Query != nil {
		query = *binding.Query
	}
	if request.Query != nil {
		if !slices.Contains(caps, "query") {
			return fail("forbidden", "Query capability is not granted")
		}
		// Additional filters narrow the authored binding. They cannot remove its scope.
		supplied := *request.Query
		if query.Where != nil && supplied.Where != nil {
			supplied.Where = &shardv2.Predicate{And: []shardv2.Predicate{*query.Where, *supplied.Where}}
		} else if query.Where != nil {
			supplied.Where = query.Where
		}
		if supplied.OrderBy == nil {
			supplied.OrderBy = query.OrderBy
		}
		if supplied.Limit == 0 {
			supplied.Limit = query.Limit
		}
		query = supplied
	}
	if query.Limit == 0 {
		query.Limit = shardv2.DefaultLimit
		if definition.Query.MaxLimit > 0 && definition.Query.MaxLimit < query.Limit {
			query.Limit = definition.Query.MaxLimit
		}
	}
	if definition.Kind == "singleton" {
		query.Limit = 1
		if request.ID != "" && request.ID != "value" {
			return fail("bad-request", "Singleton record ID is value")
		}
	}
	query, err = shardv2.NormalizeQuery(query)
	if err != nil {
		return fail("bad-request", "Invalid resource query")
	}
	if err := shardv2.ValidateQuery(definition, query); err != nil {
		return fail("bad-request", "Invalid resource query")
	}
	output, projectionErr := shardv2.ProjectSchema(definition.Schema, binding.Select)
	if projectionErr != nil {
		return fail("invalid-schema", "Invalid output projection")
	}
	// A caller must not filter/sort by fields hidden by its binding projection.
	if err := shardv2.ValidateQuery(shardv2.Resource{Schema: output, Operations: definition.Operations, Query: definition.Query}, query); err != nil {
		return fail("forbidden", "Query references a field outside the binding projection")
	}
	view = ResourceView{Namespace: ResourceNamespace{UserID: principal.UserID, ActorKey: resourcePrincipalKey(principal), ShardID: target.ShardID, Environment: target.Environment, DatasetID: definition.Source.Dataset, Generation: release.Generation, ContractHash: release.Hash}, Definition: definition, Params: params, Query: query, ID: request.ID, Select: binding.Select, OutputSchema: output}
	noCursor := query
	noCursor.Cursor = nil
	view.ViewHash = resourceHash([]any{view.Namespace, target.Audience, binding.Resource, params, noCursor, request.ID, binding.Select, principal.Scopes})
	view.URI = "shard://" + target.ShardID + "/resources/" + binding.Resource + "?view=" + view.ViewHash
	if err := provider.Authorize(ctx, view); err != nil {
		return view, descriptor, provider, err
	}
	descriptor = ResourceDescriptor{Kind: definition.Kind, SchemaVersion: definition.SchemaVersion, Schema: output, Capabilities: caps, Delivery: "snapshots", Limit: query.Limit}
	if slices.Contains(caps, "observe") {
		descriptor.Observation = provider.Profile().Observation
	}
	return view, descriptor, provider, nil
}

func (s *shardResourceService) Describe(ctx context.Context, target ResourceTarget, request ResourceRequest) (ResourceDescriptor, error) {
	if err := s.admit(ctx); err != nil {
		return ResourceDescriptor{}, err
	}
	_, descriptor, _, err := s.resolve(ctx, target, request, "snapshot")
	return descriptor, err
}
func (s *shardResourceService) Read(ctx context.Context, target ResourceTarget, request ResourceRequest) (ResourceSnapshot, error) {
	if err := s.admit(ctx); err != nil {
		return ResourceSnapshot{}, err
	}
	capability := "snapshot"
	if request.Query != nil {
		capability = "query"
	}
	view, _, provider, err := s.resolve(ctx, target, request, capability)
	if err != nil {
		return ResourceSnapshot{}, err
	}
	return s.snapshot(ctx, view, provider)
}
func (s *shardResourceService) snapshot(ctx context.Context, view ResourceView, provider ResourceProvider) (ResourceSnapshot, error) {
	snapshotCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	page, err := provider.Snapshot(snapshotCtx, view)
	if err != nil {
		return ResourceSnapshot{}, err
	}
	if err := s.checkCurrentView(snapshotCtx, view); err != nil {
		return ResourceSnapshot{}, err
	}
	if len(page.Records) > view.Query.Limit {
		return ResourceSnapshot{}, ResourceFailure("invalid-schema", "Provider exceeded the view limit")
	}
	records := make([]shardv2.Record, 0, len(page.Records))
	for _, record := range page.Records {
		data, err := shardv2.DecodeJSON(record.Data)
		if err != nil || record.SchemaVersion != view.Definition.SchemaVersion {
			return ResourceSnapshot{}, ResourceFailure("invalid-schema", "Provider returned incompatible data")
		}
		if err := shardv2.ValidateData(view.Definition.Schema, data); err != nil {
			return ResourceSnapshot{}, ResourceFailure("invalid-schema", "Provider returned invalid data")
		}
		projected, err := shardv2.ProjectData(data, view.Select)
		if err != nil {
			return ResourceSnapshot{}, ResourceFailure("invalid-schema", "Provider projection failed")
		}
		record.Data, _ = json.Marshal(projected)
		records = append(records, record)
	}
	result := ResourceSnapshot{Resource: view.URI, Records: records, Complete: true, NextCursor: page.NextCursor, SourceUpdatedAt: page.SourceUpdatedAt}
	raw, _ := json.Marshal(result)
	value, err := shardv2.DecodeJSON(raw)
	if err != nil || len(raw) > shardv2.MaxJSONBytes-2048 || shardv2.ValidateProtocol("snapshot", value) != nil {
		return ResourceSnapshot{}, ResourceFailure("invalid-schema", "Provider snapshot exceeds the protocol bounds")
	}
	event := shardv2.Event{Protocol: shardv2.StreamVersion, SubscriptionID: "validation", Resource: view.URI, Epoch: "validation", Seq: "0", Op: "snapshot", Records: records, Complete: true}
	raw, _ = json.Marshal(event)
	if _, err := shardv2.ValidateEvent(raw, view.Definition, view.OutputSchema); err != nil {
		return ResourceSnapshot{}, ResourceFailure("invalid-schema", "Provider returned an invalid record envelope")
	}
	return result, nil
}

func (s *shardResourceService) Mutate(ctx context.Context, target ResourceTarget, mutation ResourceMutation) (ResourceMutationResult, error) {
	if err := s.admit(ctx); err != nil {
		return ResourceMutationResult{}, err
	}
	if !slices.Contains([]string{"insert", "update", "delete"}, mutation.Op) {
		return ResourceMutationResult{}, ResourceFailure("bad-request", "Unknown mutation operation")
	}
	// Queries cannot alter mutation addressing; updates always validate full data.
	if mutation.Query != nil {
		return ResourceMutationResult{}, ResourceFailure("bad-request", "Mutations cannot carry a query")
	}
	view, _, provider, err := s.resolve(ctx, target, mutation.ResourceRequest, mutation.Op)
	if err != nil {
		return ResourceMutationResult{}, err
	}
	resourceID := strings.Split(strings.Split(view.URI, "/resources/")[1], "?")[0]
	command := shardv2.Command{Resource: resourceID, Op: mutation.Op, ID: mutation.ID, Data: mutation.Data, BaseRevision: mutation.BaseRevision, RequestID: mutation.RequestID, ContractHash: target.ContractHash}
	if view.Definition.Kind == "singleton" && command.Op == "insert" && command.ID == "" {
		command.ID = "value"
	}
	raw, err := json.Marshal(command)
	if err != nil {
		return ResourceMutationResult{}, ResourceFailure("bad-request", "Invalid mutation")
	}
	value, err := shardv2.DecodeJSON(raw)
	if err != nil || shardv2.ValidateProtocol("command", value) != nil {
		return ResourceMutationResult{}, ResourceFailure("bad-request", "Invalid mutation command")
	}
	if command.Op != "delete" {
		data, err := shardv2.DecodeJSON(command.Data)
		if err != nil || len(command.Data) > shardv2.MaxRecordBytes || shardv2.ValidateData(view.Definition.Schema, data) != nil {
			return ResourceMutationResult{}, ResourceFailure("invalid-schema", "Mutation requires complete valid resource data")
		}
	}
	result, err := provider.Mutate(ctx, view, command)
	if err != nil {
		return ResourceMutationResult{}, err
	}
	if result.RequestID != command.RequestID {
		return ResourceMutationResult{}, ResourceFailure("source-unavailable", "Mutation outcome is unconfirmed")
	}
	if err := s.checkCurrentView(ctx, view); err != nil {
		return ResourceMutationResult{}, err
	}
	var envelope shardv2.Record
	if command.Op == "delete" {
		if result.Tombstone == nil || result.Record != nil || result.Tombstone.ID != command.ID {
			return ResourceMutationResult{}, ResourceFailure("invalid-schema", "Invalid deletion receipt")
		}
		envelope = shardv2.Record{ID: result.Tombstone.ID, Revision: result.Tombstone.Revision, SchemaVersion: view.Definition.SchemaVersion, Data: json.RawMessage(`{}`)}
	} else {
		if result.Record == nil || result.Tombstone != nil || (command.ID != "" && result.Record.ID != command.ID) {
			return ResourceMutationResult{}, ResourceFailure("invalid-schema", "Invalid mutation receipt")
		}
		envelope = *result.Record
	}
	encoded, encodeErr := json.Marshal(envelope)
	decoded, decodeErr := shardv2.DecodeJSON(encoded)
	if encodeErr != nil || decodeErr != nil || shardv2.ValidateProtocol("record", decoded) != nil {
		return ResourceMutationResult{}, ResourceFailure("invalid-schema", "Invalid mutation receipt envelope")
	}

	// Receipts store canonical outcomes; apply the caller's current projection
	// every time, including idempotent replays, before returning any record data.
	if result.Record != nil {
		snapshot, err := s.projectMutation(ctx, view, *result.Record)
		if err != nil {
			return ResourceMutationResult{}, err
		}
		result.Record = &snapshot
	}
	return result, nil
}
func (s *shardResourceService) projectMutation(_ context.Context, view ResourceView, record shardv2.Record) (shardv2.Record, error) {
	data, err := shardv2.DecodeJSON(record.Data)
	if err != nil || record.SchemaVersion != view.Definition.SchemaVersion || shardv2.ValidateData(view.Definition.Schema, data) != nil {
		return record, ResourceFailure("invalid-schema", "Invalid mutation receipt")
	}
	projected, err := shardv2.ProjectData(data, view.Select)
	if err != nil {
		return record, err
	}
	record.Data, _ = json.Marshal(projected)
	return record, nil
}

// Long source reads must not return sensitive data after release/ownership
// revocation that occurred while the provider was fetching it.
func (s *shardResourceService) checkCurrentView(ctx context.Context, view ResourceView) error {
	rec, err := s.artifacts.Get(ctx, view.Namespace.ShardID)
	if err != nil {
		return err
	}
	if rec.Type != "app" {
		return ErrNotFound
	}
	current, err := s.releases.ActiveResourceRelease(ctx, view.Namespace.UserID, view.Namespace.ShardID, view.Namespace.Environment)
	if err != nil {
		return err
	}
	if current.Hash != view.Namespace.ContractHash || current.Generation != view.Namespace.Generation {
		return ResourceFailure("contract-changed", "Resource release changed during the request")
	}
	return nil
}
