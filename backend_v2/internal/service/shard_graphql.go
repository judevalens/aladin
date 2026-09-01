package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aladin/backend_v2/internal/shardv2"
)

type ShardGraphQLRequest struct {
	OperationID string         `json:"operationId"`
	Variables   map[string]any `json:"variables,omitempty"`
}
type ShardLambdaRequest struct {
	Name  string         `json:"name"`
	Input map[string]any `json:"input,omitempty"`
}
type RuntimeCapabilityRequest struct {
	ReleaseHash string         `json:"releaseHash"`
	Handler     string         `json:"handler"`
	Capability  string         `json:"capability"`
	Input       map[string]any `json:"input"`
	ScopeToken  string         `json:"scopeToken"`
}
type ShardGraphQLService interface {
	Enabled() bool
	Execute(context.Context, ResourceTarget, ShardGraphQLRequest) (json.RawMessage, error)
	InvokeLambda(context.Context, ResourceTarget, ShardLambdaRequest) (json.RawMessage, error)
	Capability(context.Context, string, RuntimeCapabilityRequest) (any, error)
}

type shardGraphQLService struct {
	releases  ShardReleaseService
	resources ShardResourceService
	url       string
	secret    []byte
	client    *http.Client
}

type runtimeScope struct {
	UserID      string       `json:"userId"`
	ShardID     string       `json:"shardId"`
	Environment BuildChannel `json:"environment"`
	ReleaseHash string       `json:"releaseHash"`
	Audience    string       `json:"audience"`
	ExpiresAt   int64        `json:"expiresAt"`
}

func NewShardGraphQLService(releases ShardReleaseService, resources ShardResourceService, runtimeURL, secret string) ShardGraphQLService {
	return &shardGraphQLService{releases: releases, resources: resources, url: strings.TrimRight(runtimeURL, "/"), secret: []byte(secret), client: &http.Client{Timeout: 35 * time.Second}}
}
func (s *shardGraphQLService) Enabled() bool {
	return s.url != "" && len(s.secret) >= 32 && s.releases != nil && s.resources != nil
}

func (s *shardGraphQLService) Execute(ctx context.Context, target ResourceTarget, request ShardGraphQLRequest) (json.RawMessage, error) {
	return s.invoke(ctx, target, "/v1/graphql/execute", map[string]any{"operationId": request.OperationID, "variables": request.Variables})
}
func (s *shardGraphQLService) InvokeLambda(ctx context.Context, target ResourceTarget, request ShardLambdaRequest) (json.RawMessage, error) {
	return s.invoke(ctx, target, "/v1/lambdas/invoke", map[string]any{"name": request.Name, "input": request.Input})
}

func (s *shardGraphQLService) invoke(ctx context.Context, target ResourceTarget, endpoint string, payload map[string]any) (json.RawMessage, error) {
	if !s.Enabled() {
		return nil, ResourceFailure("unsupported-capability", "Shard GraphQL runtime is disabled")
	}
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	release, err := s.releases.Active(ctx, target.ShardID, target.Environment)
	if err != nil {
		return nil, err
	}
	if target.ContractHash == "" || target.ContractHash != release.Hash {
		return nil, ResourceFailure("contract-changed", "Shard release changed")
	}
	bundle, runtimeManifest := release.Files["resolver.bundle.mjs"], release.Files["runtime-manifest.json"]
	if len(bundle) == 0 || len(runtimeManifest) == 0 {
		return nil, ResourceFailure("unsupported-capability", "Release has no authored runtime")
	}
	var manifest map[string]any
	if json.Unmarshal(runtimeManifest, &manifest) != nil {
		return nil, ResourceFailure("invalid-schema", "Runtime manifest is invalid")
	}
	scopeKey := runtimeScopeKey(principal.UserID, target.ShardID, target.Environment)
	scope := runtimeScope{UserID: principal.UserID, ShardID: target.ShardID, Environment: target.Environment, ReleaseHash: release.Hash, Audience: target.Audience, ExpiresAt: time.Now().Add(5 * time.Minute).Unix()}
	scopeToken, err := s.signScope(scope)
	if err != nil {
		return nil, err
	}
	prepare := map[string]any{"scopeKey": scopeKey, "releaseHash": release.Hash, "bundle": string(bundle), "schema": string(release.Files["schema.graphql"]), "manifest": manifest}
	if _, err := s.call(ctx, "/v1/releases/prepare", prepare); err != nil {
		return nil, err
	}
	if _, err := s.call(ctx, "/v1/releases/activate", map[string]any{"scopeKey": scopeKey, "releaseHash": release.Hash}); err != nil {
		return nil, err
	}
	payload["scopeKey"], payload["releaseHash"], payload["scopeToken"] = scopeKey, release.Hash, scopeToken
	if endpoint == "/v1/graphql/execute" {
		payload["exposure"] = target.Audience
	}
	return s.call(ctx, endpoint, payload)
}

func (s *shardGraphQLService) call(ctx context.Context, path string, payload any) (json.RawMessage, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, ResourceFailure("bad-request", "Shard runtime payload is invalid")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+string(s.secret))
	response, err := s.client.Do(req)
	if err != nil {
		return nil, ResourceFailure("source-unavailable", "Shard runtime is unavailable")
	}
	defer response.Body.Close()
	result, err := io.ReadAll(io.LimitReader(response.Body, shardv2.MaxJSONBytes+1))
	if err != nil {
		return nil, err
	}
	if len(result) > shardv2.MaxJSONBytes {
		return nil, ResourceFailure("source-unavailable", "Shard runtime response is too large")
	}
	if response.StatusCode >= 300 {
		var failure struct{ Code, Error string }
		_ = json.Unmarshal(result, &failure)
		code := "invalid-schema"
		switch strings.ToUpper(failure.Code) {
		case "FORBIDDEN":
			code = "forbidden"
		case "NOT_FOUND":
			code = "not-found"
		case "RELEASE_CHANGED":
			code = "contract-changed"
		case "TIMEOUT", "EXECUTION_FAILED":
			code = "source-unavailable"
		}
		if failure.Error == "" {
			failure.Error = "Shard runtime request failed"
		}
		return nil, ResourceFailure(code, failure.Error)
	}
	if _, err := shardv2.DecodeJSON(result); err != nil {
		return nil, ResourceFailure("source-unavailable", "Shard runtime returned invalid JSON")
	}
	return result, nil
}

func runtimeScopeKey(userID, shardID string, environment BuildChannel) string {
	sum := sha256.Sum256([]byte(userID + "\x00" + shardID + "\x00" + string(environment)))
	return hex.EncodeToString(sum[:])
}
func (s *shardGraphQLService) signScope(scope runtimeScope) (string, error) {
	raw, err := json.Marshal(scope)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (s *shardGraphQLService) verifyScope(token string) (runtimeScope, error) {
	var scope runtimeScope
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return scope, ErrForbidden
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return scope, ErrForbidden
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return scope, ErrForbidden
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write(raw)
	if !hmac.Equal(signature, mac.Sum(nil)) || json.Unmarshal(raw, &scope) != nil || time.Now().Unix() > scope.ExpiresAt {
		return scope, ErrForbidden
	}
	return scope, nil
}

func (s *shardGraphQLService) Capability(ctx context.Context, bearer string, request RuntimeCapabilityRequest) (any, error) {
	if s.resources == nil || len(s.secret) < 32 || !strings.HasPrefix(bearer, "Bearer ") || !hmac.Equal([]byte(strings.TrimPrefix(bearer, "Bearer ")), s.secret) {
		return nil, ErrForbidden
	}
	scope, err := s.verifyScope(request.ScopeToken)
	if err != nil || scope.ReleaseHash != request.ReleaseHash {
		return nil, ErrForbidden
	}
	parts := strings.SplitN(request.Capability, ":", 2)
	if len(parts) != 2 {
		return nil, ResourceFailure("bad-request", "Invalid runtime capability")
	}
	principal := Principal{UserID: scope.UserID, ActorType: ActorTypeUserSession, ActorID: "shard-runtime", Scopes: []string{ScopeArtifactsRead, ScopeArtifactsWrite}}
	capCtx := WithPrincipal(ctx, principal)
	target := ResourceTarget{ShardID: scope.ShardID, Environment: scope.Environment, Audience: scope.Audience, ContractHash: scope.ReleaseHash}
	release, err := s.releases.Active(capCtx, scope.ShardID, scope.Environment)
	if err != nil || release.Hash != scope.ReleaseHash {
		return nil, ErrForbidden
	}
	var contract shardv2.Contract
	if json.Unmarshal(release.Source, &contract) != nil {
		return nil, ErrForbidden
	}
	var granted []string
	if strings.HasPrefix(request.Handler, "lambda:") {
		granted = contract.Lambdas[strings.TrimPrefix(request.Handler, "lambda:")].Capabilities
	} else if contract.GraphQL != nil {
		granted = contract.GraphQL.Resolvers[request.Handler].Capabilities
	}
	allowed := false
	for _, capability := range granted {
		if capability == request.Capability {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, ErrForbidden
	}
	resourceRequest := ResourceRequest{Binding: parts[0]}
	if inputs, ok := request.Input["inputs"].(map[string]any); ok {
		resourceRequest.Inputs = inputs
	}
	if id, ok := request.Input["id"].(string); ok {
		resourceRequest.ID = id
	}
	if query, ok := request.Input["query"]; ok {
		raw, _ := json.Marshal(query)
		var parsed shardv2.Query
		if json.Unmarshal(raw, &parsed) != nil {
			return nil, ResourceFailure("bad-request", "Invalid runtime query")
		}
		resourceRequest.Query = &parsed
	}
	switch parts[1] {
	case "snapshot", "query":
		return s.resources.Read(capCtx, target, resourceRequest)
	case "insert", "update", "delete":
		raw, _ := json.Marshal(request.Input)
		var mutation ResourceMutation
		if err := json.Unmarshal(raw, &mutation); err != nil {
			return nil, err
		}
		mutation.Binding = parts[0]
		mutation.Inputs = resourceRequest.Inputs
		mutation.Query = resourceRequest.Query
		mutation.Op = parts[1]
		return s.resources.Mutate(capCtx, target, mutation)
	default:
		return nil, fmt.Errorf("unsupported runtime capability %q", parts[1])
	}
}

var _ ShardGraphQLService = (*shardGraphQLService)(nil)
