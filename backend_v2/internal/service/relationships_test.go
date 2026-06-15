package service

import (
	"context"
	"errors"
	"testing"
)

type fakeRelStore struct {
	created []Relationship
	list    []Relationship
}

func (f *fakeRelStore) Create(_ context.Context, rel Relationship) (Relationship, error) {
	rel.ID = "id-1"
	f.created = append(f.created, rel)
	return rel, nil
}
func (f *fakeRelStore) ListForNode(_ context.Context, _ string, _ string, _ string) ([]Relationship, error) {
	return f.list, nil
}
func (f *fakeRelStore) Delete(_ context.Context, _ string, _ string) error { return nil }

func isBadRequest(err error) bool {
	var b BadRequest
	return errors.As(err, &b)
}

// TestRelationshipServiceValidatesAndScopes covers the service's two jobs:
// validate input (kind/type/ids) and stamp the principal's userID before
// delegating to the store.
func TestRelationshipServiceValidatesAndScopes(t *testing.T) {
	store := &fakeRelStore{}
	svc := NewRelationshipService(store)
	ctx := WithPrincipal(context.Background(), Principal{UserID: "user-42"})

	// Happy path: valid edge persists with the principal's userID stamped.
	out, err := svc.Create(ctx, Relationship{SrcKind: "artifact", SrcID: "a1", DstKind: "record", DstID: "r1", RelType: "cites"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if out.ID == "" {
		t.Fatalf("expected a stored id")
	}
	if len(store.created) != 1 || store.created[0].UserID != "user-42" {
		t.Fatalf("service must stamp the principal's userID; got %+v", store.created)
	}

	// Bad kind / bad type / missing id → BadRequest, never persisted.
	bad := []Relationship{
		{SrcKind: "bogus", SrcID: "a", DstKind: "record", DstID: "r", RelType: "cites"},
		{SrcKind: "artifact", SrcID: "a", DstKind: "record", DstID: "r", RelType: "nope"},
		{SrcKind: "artifact", SrcID: "", DstKind: "record", DstID: "r", RelType: "cites"},
	}
	for i, b := range bad {
		if _, err := svc.Create(ctx, b); !isBadRequest(err) {
			t.Fatalf("invalid edge #%d should be BadRequest, got %v", i, err)
		}
	}
	if len(store.created) != 1 {
		t.Fatalf("invalid edges must not persist; store has %d", len(store.created))
	}

	// Missing principal → error, not a panic.
	if _, err := svc.Create(context.Background(), Relationship{SrcKind: "artifact", SrcID: "a", DstKind: "record", DstID: "r", RelType: "cites"}); err == nil {
		t.Fatalf("missing principal should error")
	}

	// ListForNode validates kind too.
	if _, err := svc.ListForNode(ctx, "bogus", "x"); !isBadRequest(err) {
		t.Fatalf("list with bad kind should be BadRequest, got %v", err)
	}
}
