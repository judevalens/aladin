package service

import (
	"errors"

	"aladin/backend_v2/internal/shardresource"
)

// Compatibility aliases preserve the established service API while the
// authoritative contracts live in internal/shardresource.
type ResourceTarget = shardresource.Target
type ResourceRequest = shardresource.ResourceRequest
type ResourceDescriptor = shardresource.Descriptor
type ResourceSnapshot = shardresource.Snapshot
type ResourceMutation = shardresource.Mutation
type ResourceTombstone = shardresource.Tombstone
type ResourceMutationResult = shardresource.MutationResult
type ResourceError = shardresource.Error
type ResourceRelease = shardresource.ResourceRelease
type ResourceReleaseReader = shardresource.ReleaseReader
type ResourceNamespace = shardresource.Namespace
type ResourceView = shardresource.View
type ResourcePage = shardresource.Page
type ResourceProvider = shardresource.Provider
type ResourceChangeObserver = shardresource.ChangeObserver
type ResourceSubscriptionIdentity = shardresource.SubscriptionIdentity
type ResourceStreamMessage = shardresource.StreamMessage
type ResourceSubscription = shardresource.Subscription
type ShardResourceService = shardresource.Service
type ResourceServiceOptions = shardresource.Options

func ResourceFailure(code, message string) error { return shardresource.Failure(code, message) }

func ResourceErrorCode(err error) string {
	if code := shardresource.ErrorCode(err, ""); code != "" {
		return code
	}
	if errors.Is(err, ErrUnauthenticated) || errors.Is(err, ErrForbidden) {
		return "forbidden"
	}
	if errors.Is(err, ErrNotFound) {
		return "not-found"
	}
	return "source-unavailable"
}
