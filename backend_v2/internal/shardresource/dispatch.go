package shardresource

import (
	"context"
	"encoding/json"
	"strings"

	"aladin/backend_v2/internal/shardv2"
)

type BridgeCommand struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func IsDispatchable(method string) bool {
	switch method {
	case "hello", "resource.describe", "resource.read", "resource.query", "resource.insert", "resource.update", "resource.delete":
		return true
	default:
		return false
	}
}

func ParseBridge(raw []byte) (BridgeCommand, error) {
	var command BridgeCommand
	value, err := shardv2.DecodeJSON(raw)
	if err != nil || shardv2.ValidateProtocol("bridge-request", value) != nil {
		return command, Failure("bad-request", "Invalid bridge/2 request")
	}
	err = json.Unmarshal(raw, &command)
	return command, err
}

// Dispatch is the one command/query dispatch path shared by HTTP and preview.
// Subscription and host-local commands stay in their stateful transports.
func Dispatch(ctx context.Context, svc Service, target Target, command BridgeCommand) (any, error) {
	if !IsDispatchable(command.Method) {
		return nil, Failure("unsupported-capability", "Unknown resource command")
	}
	var request ResourceRequest
	if err := json.Unmarshal(command.Params, &request); err != nil {
		return nil, err
	}
	switch command.Method {
	case "hello":
		return svc.Hello(ctx, target)
	case "resource.describe":
		return svc.Describe(ctx, target, request)
	case "resource.read", "resource.query":
		return svc.Read(ctx, target, request)
	case "resource.insert", "resource.update", "resource.delete":
		var mutation Mutation
		if err := json.Unmarshal(command.Params, &mutation); err != nil {
			return nil, err
		}
		mutation.Op = strings.TrimPrefix(command.Method, "resource.")
		return svc.Mutate(ctx, target, mutation)
	}
	panic("unreachable resource dispatch")
}
