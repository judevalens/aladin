package service

import (
	"aladin/backend_v2/internal/shardv2"
	"context"
	"encoding/json"
	"strings"
)

type ResourceBridgeCommand struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func ParseResourceBridge(raw []byte) (ResourceBridgeCommand, error) {
	var command ResourceBridgeCommand
	value, err := shardv2.DecodeJSON(raw)
	if err != nil || shardv2.ValidateProtocol("bridge-request", value) != nil {
		return command, ResourceFailure("bad-request", "Invalid bridge/2 request")
	}
	err = json.Unmarshal(raw, &command)
	return command, err
}
func DispatchResource(ctx context.Context, svc ShardResourceService, target ResourceTarget, command ResourceBridgeCommand) (any, error) {
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
		var mutation ResourceMutation
		if err := json.Unmarshal(command.Params, &mutation); err != nil {
			return nil, err
		}
		mutation.Op = strings.TrimPrefix(command.Method, "resource.")
		return svc.Mutate(ctx, target, mutation)
	default:
		return nil, ResourceFailure("unsupported-capability", "Unknown resource command")
	}
}
