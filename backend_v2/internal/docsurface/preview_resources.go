package docsurface

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"aladin/backend_v2/internal/safego"
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardresource"
	"aladin/backend_v2/internal/shardv2"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const previewResourceBridgeJS = `(function(){
const generation="__PREVIEW_RESOURCE_NONCE__";
window.__aladinPreviewGeneration=generation;
window.addEventListener("message",function(event){
 if(window.__aladinPreviewGeneration!==generation||event.source!==window)return;
 const m=event.data;if(!m||m.aladin!=="bridge/2"||m.type!=="request")return;
 window.aladinPreviewResource(JSON.stringify({generation,request:m}));
});})();`

func previewResourceRelease(build service.BuildResult) service.ResourceRelease {
	sum := sha256.Sum256(build.Contract)
	return service.ResourceRelease{Source: build.Contract, Hash: hex.EncodeToString(sum[:]), BuildID: build.BuildID}
}

func (m *PreviewSessions) configureResourcePreview(caller context.Context, session *previewSession, pageID string, build service.BuildResult, nonce string) error {
	session.resourceMu.Lock()
	if session.resourceCancel != nil {
		session.resourceCancel()
	}
	session.resourceQueue = nil
	session.resourceMu.Unlock()
	if len(build.Contract) == 0 {
		return nil
	}
	principal, err := service.RequirePrincipal(caller)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(service.WithPrincipal(session.tabCtx, principal))
	queue := make(chan string, 32)
	if err := chromedp.Run(ctx, cdpruntime.AddBinding("aladinPreviewResource")); err != nil {
		cancel()
		return err
	}
	session.resourceMu.Lock()
	session.resourceQueue, session.resourceCancel = queue, cancel
	session.resourceMu.Unlock()
	target := service.ResourceTarget{ShardID: pageID, Environment: service.ChannelDraft, Audience: "app", ContractHash: previewResourceRelease(build).Hash}
	var sendMu sync.Mutex
	send := func(value any) {
		raw, err := json.Marshal(value)
		if err != nil || len(raw) > shardv2.MaxJSONBytes {
			cancel()
			return
		}
		sendMu.Lock()
		defer sendMu.Unlock()
		op, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		_ = chromedp.Run(op, chromedp.Evaluate("if(window.__aladinPreviewGeneration==="+jsString(nonce)+"){window.postMessage("+string(raw)+",'*')}", nil))
	}
	reply := func(id int64, value any, err error) {
		result := map[string]any{"aladin": "bridge/2", "type": "response", "id": id, "ok": err == nil}
		if err == nil {
			result["data"] = value
		} else {
			result["code"] = service.ResourceErrorCode(err)
			result["error"] = "Preview resource request failed"
		}
		send(result)
	}
	safego.Go("shard.preview.resources", func() {
		subscriptions := map[string]service.ResourceSubscription{}
		defer func() {
			for _, sub := range subscriptions {
				sub.Close()
			}
		}()
		for {
			var raw string
			select {
			case <-ctx.Done():
				return
			case raw = <-queue:
			}
			if len(raw) > shardv2.MaxJSONBytes {
				cancel()
				return
			}
			var envelope struct {
				Generation string          `json:"generation"`
				Request    json.RawMessage `json:"request"`
			}
			if json.Unmarshal([]byte(raw), &envelope) != nil || envelope.Generation != nonce {
				continue
			}
			request, err := shardresource.ParseBridge(envelope.Request)
			if err != nil {
				reply(request.ID, nil, err)
				continue
			}
			switch request.Method {
			case "theme.get":
				// Theme is already stamped. No storage or synthetic external data.
				var theme string
				_ = chromedp.Run(ctx, chromedp.Evaluate("document.documentElement.dataset.theme||'dark'", &theme))
				reply(request.ID, map[string]any{"theme": theme}, nil)
			case "resource.subscribe":
				if len(subscriptions) >= 32 {
					reply(request.ID, nil, service.ResourceFailure("rate-limited", "Preview subscription limit"))
					continue
				}
				var params service.ResourceRequest
				_ = json.Unmarshal(request.Params, &params)
				sub, err := m.resources.Subscribe(ctx, target, params)
				if err != nil {
					reply(request.ID, nil, err)
					continue
				}
				subscriptions[sub.Identity.SubscriptionID] = sub
				reply(request.ID, sub.Identity, nil)
				safego.Go("shard.preview.forward", func() {
					defer sub.Close()
					for message := range sub.Events {
						channel := "resource.event"
						var data any = message.Event
						if message.Error != nil {
							channel = "resource.error"
							data = map[string]any{"subscriptionId": sub.Identity.SubscriptionID, "resource": sub.Identity.Resource, "epoch": sub.Identity.Epoch, "code": message.Error.Code, "message": message.Error.Message}
						}
						send(map[string]any{"aladin": "bridge/2", "type": "push", "channel": channel, "data": data})
					}
				})
			case "resource.unsubscribe":
				var params struct {
					SubscriptionID string `json:"subscriptionId"`
				}
				_ = json.Unmarshal(request.Params, &params)
				if sub, ok := subscriptions[params.SubscriptionID]; ok {
					sub.Close()
					delete(subscriptions, params.SubscriptionID)
				}
				reply(request.ID, true, nil)
			case "graphql.execute":
				if m.graphql == nil || !m.graphql.Enabled() {
					reply(request.ID, nil, service.ResourceFailure("unsupported-capability", "Shard GraphQL runtime is unavailable"))
					continue
				}
				var params service.ShardGraphQLRequest
				if err := json.Unmarshal(request.Params, &params); err != nil {
					reply(request.ID, nil, service.ResourceFailure("bad-request", "Invalid GraphQL operation"))
					continue
				}
				op, stop := context.WithTimeout(ctx, 35*time.Second)
				raw, err := m.graphql.Execute(op, target, params)
				stop()
				var value any
				if err == nil {
					value, err = shardv2.DecodeJSON(raw)
				}
				reply(request.ID, value, err)
			case "lambda.invoke":
				if m.graphql == nil || !m.graphql.Enabled() {
					reply(request.ID, nil, service.ResourceFailure("unsupported-capability", "Shard lambda runtime is unavailable"))
					continue
				}
				var params service.ShardLambdaRequest
				if err := json.Unmarshal(request.Params, &params); err != nil {
					reply(request.ID, nil, service.ResourceFailure("bad-request", "Invalid lambda invocation"))
					continue
				}
				op, stop := context.WithTimeout(ctx, 35*time.Second)
				raw, err := m.graphql.InvokeLambda(op, target, params)
				stop()
				var value any
				if err == nil {
					value, err = shardv2.DecodeJSON(raw)
				}
				reply(request.ID, value, err)
			default:
				op, stop := context.WithTimeout(ctx, 10*time.Second)
				value, err := shardresource.Dispatch(op, m.resources, target, request)
				stop()
				reply(request.ID, value, err)
			}
		}
	})
	return nil
}
