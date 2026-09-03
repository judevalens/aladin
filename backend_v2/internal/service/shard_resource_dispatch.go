package service

import (
	"context"

	"aladin/backend_v2/internal/shardresource"
)

type ResourceBridgeCommand = shardresource.BridgeCommand

func ParseResourceBridge(raw []byte) (ResourceBridgeCommand, error) {
	return shardresource.ParseBridge(raw)
}

func DispatchResource(ctx context.Context, svc ShardResourceService, target ResourceTarget, command ResourceBridgeCommand) (any, error) {
	return shardresource.Dispatch(ctx, svc, target, command)
}
