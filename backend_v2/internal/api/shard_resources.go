package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"aladin/backend_v2/internal/safego"
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardv2"
	"github.com/coder/websocket"
)

// The host carries its pinned release hash separately from the iframe's
// request. Reuse bridge/2 envelopes on HTTP and WS; no second command protocol.
const shardContractHeader = "X-Shard-Contract"

type resourceBridgeRequest struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type resourceBridgeResponse struct {
	Aladin string `json:"aladin"`
	Type   string `json:"type"`
	ID     int64  `json:"id"`
	OK     bool   `json:"ok"`
	Data   any    `json:"data,omitempty"`
	Code   string `json:"code,omitempty"`
	Error  string `json:"error,omitempty"`
}

func resourceResponse(id int64, data any, err error) resourceBridgeResponse {
	response := resourceBridgeResponse{Aladin: shardv2.BridgeVersion, Type: "response", ID: id, OK: err == nil, Data: data}
	if err != nil {
		response.Data = nil
		response.Code = service.ResourceErrorCode(err)
		response.Error = "Resource request failed"
		if failure, ok := err.(*service.ResourceError); ok {
			response.Error = failure.Message
		}
		if response.Code == "source-unavailable" {
			response.Error = "Resource source is unavailable"
		}
	}
	return response
}
func decodeResourceBridge(raw []byte) (resourceBridgeRequest, error) {
	var request resourceBridgeRequest
	value, err := shardv2.DecodeJSON(raw)
	if err != nil || shardv2.ValidateProtocol("bridge-request", value) != nil {
		return request, service.ResourceFailure("bad-request", "Invalid bridge/2 request")
	}
	err = json.Unmarshal(raw, &request)
	return request, err
}
func resourceTarget(r *http.Request) service.ResourceTarget {
	audience := "app"
	if principal, ok := service.PrincipalFromContext(r.Context()); ok && principal.ActorType == service.ActorTypeIntegrationToken {
		audience = "agent"
	}
	return service.ResourceTarget{ShardID: r.PathValue("id"), Environment: service.BuildChannel(r.PathValue("environment")), Audience: audience, ContractHash: r.Header.Get(shardContractHeader)}
}
func (s *Server) handleShardResourceRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.deps.ShardResources() == nil {
		writeJSON(w, http.StatusNotFound, resourceResponse(0, nil, service.ErrNotFound))
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, shardv2.MaxJSONBytes))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, resourceResponse(0, nil, service.ResourceFailure("bad-request", "Request body is too large")))
		return
	}
	request, err := decodeResourceBridge(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, resourceResponse(request.ID, nil, err))
		return
	}
	target := resourceTarget(r)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	var result any
	var params service.ResourceRequest
	if err = json.Unmarshal(request.Params, &params); err == nil {
		switch request.Method {
		case "hello":
			result, err = s.deps.ShardResources().Hello(ctx, target)
		case "resource.describe":
			result, err = s.deps.ShardResources().Describe(ctx, target, params)
		case "resource.read", "resource.query":
			result, err = s.deps.ShardResources().Read(ctx, target, params)
		case "resource.insert", "resource.update", "resource.delete":
			var mutation service.ResourceMutation
			if err = json.Unmarshal(request.Params, &mutation); err == nil {
				mutation.Op = strings.TrimPrefix(request.Method, "resource.")
				result, err = s.deps.ShardResources().Mutate(ctx, target, mutation)
			}
		default:
			err = service.ResourceFailure("unsupported-capability", "Use the resource socket for subscriptions; theme is host-local")
		}
	}
	status := http.StatusOK
	if err != nil {
		switch service.ResourceErrorCode(err) {
		case "forbidden":
			status = http.StatusForbidden
		case "not-found":
			status = http.StatusNotFound
		case "conflict", "contract-changed", "stale-cursor":
			status = http.StatusConflict
		case "quota", "rate-limited":
			status = http.StatusTooManyRequests
		case "source-unavailable":
			status = http.StatusServiceUnavailable
		default:
			status = http.StatusBadRequest
		}
	}
	writeJSON(w, status, resourceResponse(request.ID, result, err))
}

// A socket is one shard session with at most 32 logical subscriptions. Socket
// writes are serialized; each write has a deadline and no unbounded queue.
func (s *Server) handleShardResourceSocket(w http.ResponseWriter, r *http.Request) {
	svc := s.deps.ShardResources()
	if svc == nil {
		writeJSON(w, http.StatusNotFound, resourceResponse(0, nil, service.ErrNotFound))
		return
	}
	target := resourceTarget(r)
	if target.ContractHash == "" {
		target.ContractHash = r.URL.Query().Get("contractHash")
	}
	shared := r.URL.Path == "/api/shard-resources/ws"
	principal, err := service.RequirePrincipal(r.Context())
	if err != nil || principal.ActorType == service.ActorTypeContentToken || !service.HasScope(r.Context(), service.ScopeArtifactsRead) {
		writeJSON(w, http.StatusForbidden, resourceResponse(0, nil, service.ErrForbidden))
		return
	}
	var hello map[string]any
	if !shared {
		hello, err = svc.Hello(r.Context(), target)
	}
	if !shared && (err != nil || target.ContractHash == "" || hello["contractHash"] != target.ContractHash) {
		writeJSON(w, http.StatusConflict, resourceResponse(0, nil, service.ResourceFailure("contract-changed", "A current release hash is required")))
		return
	}
	// Keep the normal same-origin handshake protection. Cross-origin hosts must
	// use a same-origin proxy; an opaque shard iframe cannot open this socket.
	accept := &websocket.AcceptOptions{}
	// Native hosts have a different origin. Only explicitly known host origins
	// may cross this boundary, and an actual bearer (not a cookie) is required.
	origin := r.Header.Get("Origin")
	if origin == "null" {
		http.Error(w, "Opaque origins cannot open resource sockets", http.StatusForbidden)
		return
	}
	if r.URL.Query().Get("access_token") != "" || strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		for _, allowed := range strings.Split("http://localhost:4173,tauri://localhost,http://tauri.localhost,https://tauri.localhost,"+os.Getenv("SHARD_HOST_ORIGINS"), ",") {
			if origin != "" && origin == strings.TrimSpace(allowed) {
				accept.InsecureSkipVerify = true
			}
		}
	}
	conn, err := websocket.Accept(w, r, accept)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(shardv2.MaxJSONBytes)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	var writeMu sync.Mutex
	send := func(value any) error {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if len(data) > shardv2.MaxJSONBytes {
			return service.ResourceFailure("resync-required", "Resource frame is too large")
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		writeCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		return conn.Write(writeCtx, websocket.MessageText, data)
	}
	var mu sync.Mutex
	active := map[string]service.ResourceSubscription{}
	subscriptionLimit := 32
	if shared {
		subscriptionLimit = 128
	}
	defer func() {
		mu.Lock()
		defer mu.Unlock()
		for _, sub := range active {
			sub.Close()
		}
	}()
	// Authentication on the upgrade alone is insufficient for a long-lived
	// socket. Recheck the exact credential against the auth store periodically.
	credential := s.resourceSocketCredential(r)
	if s.deps.Auth() != nil {
		original, _ := service.RequirePrincipal(ctx)
		safego.Go("shard.resource.auth", func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
				current, err := s.deps.Auth().ResolveBearerToken(ctx, credential)
				if err != nil || current.UserID != original.UserID || current.ActorType != original.ActorType || current.ActorID != original.ActorID || !reflect.DeepEqual(current.Scopes, original.Scopes) {
					_ = conn.Close(websocket.StatusPolicyViolation, "Resource session expired")
					cancel()
					return
				}
			}
		})
	}
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return
		}
		requestTarget := target
		requestRaw := raw
		if shared {
			value, decodeErr := shardv2.DecodeJSON(raw)
			if decodeErr != nil || shardv2.ValidateProtocol("host-request", value) != nil {
				if send(resourceResponse(0, nil, service.ResourceFailure("bad-request", "Invalid host request"))) != nil {
					return
				}
				continue
			}
			var envelope struct {
				Target struct {
					ShardID     string               `json:"shardId"`
					Environment service.BuildChannel `json:"environment"`
					Hash        string               `json:"contractHash"`
				} `json:"target"`
				Request json.RawMessage `json:"request"`
			}
			_ = json.Unmarshal(raw, &envelope)
			requestTarget.ShardID, requestTarget.Environment, requestTarget.ContractHash = envelope.Target.ShardID, envelope.Target.Environment, envelope.Target.Hash
			requestRaw = envelope.Request
		}
		request, err := decodeResourceBridge(requestRaw)
		if err != nil {
			if send(resourceResponse(0, nil, err)) != nil {
				return
			}
			continue
		}
		switch request.Method {
		case "resource.subscribe":
			mu.Lock()
			count := len(active)
			mu.Unlock()
			if count >= subscriptionLimit {
				if send(resourceResponse(request.ID, nil, service.ResourceFailure("rate-limited", "Shard session subscription limit exceeded"))) != nil {
					return
				}
				continue
			}
			var params service.ResourceRequest
			_ = json.Unmarshal(request.Params, &params)
			sub, err := svc.Subscribe(ctx, requestTarget, params)
			if err != nil {
				if send(resourceResponse(request.ID, nil, err)) != nil {
					return
				}
				continue
			}
			mu.Lock()
			active[sub.Identity.SubscriptionID] = sub
			mu.Unlock()
			if send(resourceResponse(request.ID, sub.Identity, nil)) != nil {
				sub.Close()
				return
			}
			safego.Go("shard.resource.forward", func() {
				defer sub.Close()
				defer func() { mu.Lock(); delete(active, sub.Identity.SubscriptionID); mu.Unlock() }()
				for message := range sub.Events {
					channel := "resource.event"
					var payload any = message.Event
					if message.Error != nil {
						channel = "resource.error"
						payload = map[string]any{"subscriptionId": sub.Identity.SubscriptionID, "resource": sub.Identity.Resource, "epoch": sub.Identity.Epoch, "code": message.Error.Code, "message": message.Error.Message}
					}
					if err := send(map[string]any{"aladin": shardv2.BridgeVersion, "type": "push", "channel": channel, "data": payload}); err != nil {
						cancel()
						_ = conn.CloseNow()
						return
					}
				}
			})
		case "resource.unsubscribe":
			var params struct {
				SubscriptionID string `json:"subscriptionId"`
			}
			_ = json.Unmarshal(request.Params, &params)
			mu.Lock()
			sub, ok := active[params.SubscriptionID]
			delete(active, params.SubscriptionID)
			mu.Unlock()
			if ok {
				sub.Close()
			}
			if send(resourceResponse(request.ID, true, nil)) != nil {
				return
			}
		default:
			if send(resourceResponse(request.ID, nil, service.ResourceFailure("unsupported-capability", "Socket accepts only subscribe/unsubscribe"))) != nil {
				return
			}
		}
	}
}
func (s *Server) resourceSocketCredential(r *http.Request) string {
	original, _ := service.RequirePrincipal(r.Context())
	candidates := []string{}
	if cookie, err := r.Cookie(service.SessionCookieName); err == nil && cookie.Value != "" {
		candidates = append(candidates, cookie.Value)
	}
	if parts := strings.Fields(r.Header.Get("Authorization")); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		candidates = append(candidates, parts[1])
	}
	candidates = append(candidates, r.URL.Query().Get("access_token"))
	if s.deps.Auth() == nil {
		return ""
	}
	for _, token := range candidates {
		if token == "" {
			continue
		}
		p, err := s.deps.Auth().ResolveBearerToken(r.Context(), token)
		if err == nil && p.UserID == original.UserID && p.ActorType == original.ActorType && p.ActorID == original.ActorID && p.SessionTokenHash == original.SessionTokenHash && reflect.DeepEqual(p.Scopes, original.Scopes) {
			return token
		}
	}
	return ""
}
