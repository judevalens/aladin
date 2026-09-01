package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"aladin/backend_v2/internal/shardv2"
)

type activationStoreStub struct {
	events *[]string
	err    error
}

func (*activationStoreStub) ActiveResourceRelease(context.Context, string, string, BuildChannel) (ResourceRelease, error) {
	return ResourceRelease{}, ErrNotFound
}
func (*activationStoreStub) StageResourceBuild(context.Context, string, BuildChannel, string, string, *shardv2.Compiled, map[string][]byte) error {
	return nil
}
func (s *activationStoreStub) ActivateResourceRelease(context.Context, string, BuildChannel, string, shardv2.Registry) error {
	*s.events = append(*s.events, "activate")
	return s.err
}
func (*activationStoreStub) ActiveResourceBuild(context.Context, string, BuildChannel) (ShardRelease, error) {
	return ShardRelease{}, ErrNotFound
}

type activationFenceStub struct {
	events *[]string
}

func (*activationFenceStub) ValidateResourceStage(context.Context, shardv2.Resource) error {
	return nil
}
func (s *activationFenceStub) FreezeNamespace(_ context.Context, ns ResourceNamespace, frozen bool) error {
	if ns.UserID != "owner" || ns.ShardID != "shard" || ns.Environment != ChannelPublished {
		return errors.New("unexpected namespace")
	}
	if frozen {
		*s.events = append(*s.events, "freeze")
	} else {
		*s.events = append(*s.events, "unfreeze")
	}
	return nil
}

func TestShardReleaseActivationFencesExternalDatastore(t *testing.T) {
	for _, test := range []struct {
		name      string
		activate  error
		wantError bool
	}{
		{name: "success"},
		{name: "activation failure", activate: errors.New("activation failed"), wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			events := []string{}
			store := &activationStoreStub{events: &events, err: test.activate}
			fence := &activationFenceStub{events: &events}
			service := NewShardReleaseService(store, shardv2.Registry{"shard.documents": {}}, fence)
			ctx := WithPrincipal(context.Background(), Principal{UserID: "owner", ActorType: ActorTypeUserSession, Scopes: []string{ScopeArtifactsWrite}})
			err := service.Activate(ctx, "shard", ChannelPublished, "build")
			if (err != nil) != test.wantError {
				t.Fatalf("Activate() error = %v, wantError %v", err, test.wantError)
			}
			if want := []string{"freeze", "activate", "unfreeze"}; !reflect.DeepEqual(events, want) {
				t.Fatalf("activation order = %v, want %v", events, want)
			}
		})
	}
}
