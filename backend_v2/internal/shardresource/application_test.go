package shardresource

import (
	"context"
	"errors"
	"testing"
	"time"
)

type accessContextKey struct{}

type testAccess struct{}

func (testAccess) Principal(ctx context.Context) (Principal, error) {
	principal, ok := ctx.Value(accessContextKey{}).(Principal)
	if !ok {
		return Principal{}, errors.New("unauthenticated")
	}
	return principal, nil
}
func (testAccess) RequireRead(context.Context) error        { return nil }
func (testAccess) CanWrite(context.Context) bool            { return true }
func (testAccess) RequireApp(context.Context, string) error { return nil }
func (testAccess) Forbidden() error                         { return errors.New("forbidden") }
func (testAccess) ErrorCode(error) string                   { return "forbidden" }

func TestAdmissionIsolationAndRecovery(t *testing.T) {
	service := NewService(testAccess{}, nil, nil, Options{RequestsPerSecond: 1, RequestBurst: 2}).(*shardResourceService)
	principal := Principal{UserID: "owner", ActorType: "user_session", ActorID: "session"}
	ctx := context.WithValue(context.Background(), accessContextKey{}, principal)
	for range 2 {
		if err := service.admit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if ErrorCode(service.admit(ctx), "") != "rate-limited" {
		t.Fatal("burst was not bounded")
	}
	other := context.WithValue(context.Background(), accessContextKey{}, Principal{UserID: "other", ActorType: "user_session", ActorID: "session"})
	if err := service.admit(other); err != nil {
		t.Fatalf("another account inherited the budget: %v", err)
	}
	key := resourcePrincipalKey(principal)
	bucket := service.requestBudgets[key]
	bucket.updated = time.Now().Add(-2 * time.Second)
	service.requestBudgets[key] = bucket
	if err := service.admit(ctx); err != nil {
		t.Fatalf("budget did not recover: %v", err)
	}
	content := context.WithValue(context.Background(), accessContextKey{}, Principal{UserID: "owner", ActorType: "content_token"})
	if err := service.admit(content); err == nil {
		t.Fatal("content token admitted")
	}
}
