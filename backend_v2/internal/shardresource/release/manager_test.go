package release

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"aladin/backend_v2/internal/shardresource"
	"aladin/backend_v2/internal/shardv2"
)

type managerStore struct{ events *[]string }

func (*managerStore) ActiveResourceRelease(context.Context, string, string, shardresource.Environment) (shardresource.ResourceRelease, error) {
	return shardresource.ResourceRelease{}, errors.New("not found")
}
func (*managerStore) StageResourceBuild(context.Context, string, shardresource.Environment, string, string, *shardv2.Compiled, map[string][]byte) error {
	return nil
}
func (s *managerStore) ActivateResourceRelease(context.Context, string, shardresource.Environment, string, shardv2.Registry) error {
	*s.events = append(*s.events, "activate")
	return nil
}
func (*managerStore) ActiveResourceBuild(context.Context, string, shardresource.Environment) (Build, error) {
	return Build{}, errors.New("not found")
}

type managerAuth struct{ events *[]string }

func (a managerAuth) PrincipalUserID(context.Context) (string, error) {
	*a.events = append(*a.events, "principal")
	return "owner", nil
}
func (a managerAuth) RequireRead(context.Context) error {
	*a.events = append(*a.events, "read")
	return nil
}
func (a managerAuth) RequireWrite(context.Context) error {
	*a.events = append(*a.events, "write")
	return nil
}

type managerFence struct{ events *[]string }

func (*managerFence) ValidateResourceStage(context.Context, shardv2.Resource) error { return nil }
func (f *managerFence) FreezeNamespace(_ context.Context, ns shardresource.Namespace, freeze bool) error {
	if ns.UserID != "owner" || ns.ShardID != "shard" || ns.Environment != shardresource.EnvironmentPublished {
		return errors.New("untrusted namespace")
	}
	if freeze {
		*f.events = append(*f.events, "freeze")
	} else {
		*f.events = append(*f.events, "unfreeze")
	}
	return nil
}

func TestManagerOwnsActivationSequence(t *testing.T) {
	events := []string{}
	store := &managerStore{events: &events}
	manager := NewManager(store, shardv2.Registry{"shard.documents": {}}, managerAuth{events: &events}, ErrorPolicy{Failure: shardresource.Failure, IsNotFound: func(error) bool { return true }}, &managerFence{events: &events})
	if err := manager.Activate(context.Background(), "shard", shardresource.EnvironmentPublished, "build"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"principal", "write", "freeze", "activate", "unfreeze"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("activation order = %v, want %v", events, want)
	}
}

func TestBuildIdentityCoversEveryOutput(t *testing.T) {
	one := BuildIdentity([]byte("contract"), map[string][]byte{"bundle.js": []byte("one")})
	two := BuildIdentity([]byte("contract"), map[string][]byte{"bundle.js": []byte("two")})
	if one == two || one != BuildIdentity([]byte("contract"), map[string][]byte{"bundle.js": []byte("one")}) {
		t.Fatal("build identity is not deterministic and content-sensitive")
	}
}
