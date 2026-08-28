package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	coreservice "aladin/backend_v2/internal/service"
	"github.com/google/uuid"
)

func TestSessionBoundContentTokens(t *testing.T) {
	ctx := context.Background()
	pool := mustTestPool(ctx, t)
	t.Cleanup(pool.Close)
	userID := uuid.NewString()
	seedUser(ctx, t, pool, userID)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID) })
	r := NewAuthPostgres(pool)
	now := time.Now().UTC().Truncate(time.Second)
	expiry := now.Add(30 * 24 * time.Hour)
	sessionA, sessionB := uuid.NewString(), uuid.NewString()
	contentA, contentB := uuid.NewString(), uuid.NewString()
	for _, hash := range []string{sessionA, sessionB} {
		if err := r.CreateSession(ctx, coreservice.AuthSessionRecord{UserID: userID, TokenHash: hash, ExpiresAt: expiry}); err != nil {
			t.Fatal(err)
		}
	}
	for _, pair := range [][2]string{{sessionA, contentA}, {sessionB, contentB}} {
		// Mint later than sign-in to catch accidentally adding a new TTL.
		gotExpiry, err := r.CreateContentToken(ctx, pair[1], userID, pair[0], now.Add(24*time.Hour))
		if err != nil || !gotExpiry.Equal(expiry) {
			t.Fatalf("mint expiry=%v err=%v, want %v", gotExpiry, err, expiry)
		}
		user, err := r.GetContentTokenUser(ctx, pair[1], now.Add(48*time.Hour))
		if err != nil || user.ID != userID {
			t.Fatalf("valid session after 48h: user=%+v err=%v", user, err)
		}
	}

	for _, parent := range []string{"missing", contentA} {
		if _, err := r.CreateContentToken(ctx, uuid.NewString(), userID, parent, now); !errors.Is(err, coreservice.ErrUnauthenticated) {
			t.Fatalf("non-session parent accepted: %v", err)
		}
	}
	if _, err := r.CreateContentToken(ctx, uuid.NewString(), uuid.NewString(), sessionA, now); !errors.Is(err, coreservice.ErrUnauthenticated) {
		t.Fatalf("other user's session accepted: %v", err)
	}
	if _, err := r.GetContentTokenUser(ctx, contentA, expiry); !errors.Is(err, coreservice.ErrUnauthenticated) {
		t.Fatalf("credential survived session expiry: %v", err)
	}
	if _, err := r.CreateContentToken(ctx, uuid.NewString(), userID, sessionA, expiry); !errors.Is(err, coreservice.ErrUnauthenticated) {
		t.Fatalf("expired session minted credential: %v", err)
	}

	// Logout must invalidate URLs immediately, before any opportunistic cleanup.
	if err := r.RevokeSession(ctx, sessionA); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetContentTokenUser(ctx, contentA, now); !errors.Is(err, coreservice.ErrUnauthenticated) {
		t.Fatalf("credential survived logout: %v", err)
	}
	if _, err := r.CreateContentToken(ctx, uuid.NewString(), userID, sessionA, now); !errors.Is(err, coreservice.ErrUnauthenticated) {
		t.Fatalf("revoked session minted credential: %v", err)
	}
	if err := r.DeleteExpiredContentTokens(ctx, now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM content_tokens WHERE token_hash=$1`, contentA).Scan(&count); err != nil || count != 0 {
		t.Fatalf("revoked credential not cleaned up: n=%d err=%v", count, err)
	}
	if _, err := r.GetContentTokenUser(ctx, contentB, now); err != nil {
		t.Fatalf("logout affected another session for the same user: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM user_sessions WHERE token_hash=$1`, sessionB); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM content_tokens WHERE token_hash=$1`, contentB).Scan(&count); err != nil || count != 0 {
		t.Fatalf("deleted session left a credential behind: n=%d err=%v", count, err)
	}
}
