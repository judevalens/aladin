package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAuthCredentialsRequireEmail(t *testing.T) {
	t.Parallel()

	if _, _, err := normalizeAuthCredentials(AuthCredentials{
		Email:    "admin",
		Password: "password",
	}); err == nil {
		t.Fatal("normalizeAuthCredentials error = nil, want error")
	}
}

func TestDefaultAdminPasswordHashMatchesPassword(t *testing.T) {
	t.Parallel()

	const hash = "pbkdf2_sha256$210000$YWxhZGluLWRldi1hZG1pbg$iONMwnrln6ivij4VdCYBMDNzlx8nKTrdQQhGssIkXh8"

	if !NewPasswordHasher().Compare(hash, "password") {
		t.Fatal("default admin hash did not match password")
	}
	if NewPasswordHasher().Compare(hash, "wrong-password") {
		t.Fatal("default admin hash matched wrong password")
	}
}

func TestCreateIntegrationTokenRequiresUserSession(t *testing.T) {
	t.Parallel()

	svc := NewAuthService(&fakeAuthRepo{}, NewPasswordHasher())
	if _, err := svc.CreateIntegrationToken(context.Background(), IntegrationTokenInput{
		Name:   "Claude",
		Scopes: []string{ScopeArtifactsRead},
	}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("CreateIntegrationToken error = %v, want unauthenticated", err)
	}
}

func TestCreateIntegrationTokenStoresHashAndReturnsRawTokenOnce(t *testing.T) {
	t.Parallel()

	repo := &fakeAuthRepo{}
	svc := NewAuthService(repo, NewPasswordHasher())
	ctx := WithPrincipal(context.Background(), Principal{
		UserID:    "user-1",
		ActorType: ActorTypeUserSession,
		ActorID:   "user-1",
		Email:     "user@example.com",
	})

	created, err := svc.CreateIntegrationToken(ctx, IntegrationTokenInput{
		Name:   "Claude",
		Scopes: []string{ScopeArtifactsWrite, ScopeArtifactsRead, ScopeArtifactsRead},
	})
	if err != nil {
		t.Fatalf("CreateIntegrationToken error: %v", err)
	}
	if created.Token == "" || created.Token == repo.created.TokenHash {
		t.Fatalf("raw token/hash not separated: token=%q hash=%q", created.Token, repo.created.TokenHash)
	}
	if repo.created.UserID != "user-1" {
		t.Fatalf("created user = %q, want user-1", repo.created.UserID)
	}
	if len(repo.created.Scopes) != 2 {
		t.Fatalf("scopes = %#v, want deduped scopes", repo.created.Scopes)
	}
}

func TestIntegrationTokenCannotManageIntegrationTokens(t *testing.T) {
	t.Parallel()

	svc := NewAuthService(&fakeAuthRepo{}, NewPasswordHasher())
	ctx := WithPrincipal(context.Background(), Principal{
		UserID:    "user-1",
		ActorType: ActorTypeIntegrationToken,
		ActorID:   "token-1",
		Email:     "user@example.com",
		Scopes:    []string{ScopeArtifactsRead, ScopeArtifactsWrite},
	})

	if _, err := svc.CreateIntegrationToken(ctx, IntegrationTokenInput{Name: "Nested", Scopes: []string{ScopeArtifactsRead}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("CreateIntegrationToken integration actor error = %v, want forbidden", err)
	}
	if _, err := svc.ListIntegrationTokens(ctx); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListIntegrationTokens integration actor error = %v, want forbidden", err)
	}
	if err := svc.RevokeIntegrationToken(ctx, "token-2"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("RevokeIntegrationToken integration actor error = %v, want forbidden", err)
	}
}

func TestResolveBearerTokenReturnsIntegrationPrincipal(t *testing.T) {
	t.Parallel()

	repo := &fakeAuthRepo{
		principalRecord: IntegrationTokenPrincipalRecord{
			ID:     "token-1",
			UserID: "user-1",
			Email:  "user@example.com",
			Scopes: []string{ScopeArtifactsRead},
		},
	}
	svc := NewAuthService(repo, NewPasswordHasher())

	principal, err := svc.ResolveBearerToken(context.Background(), "aladin_it_raw")
	if err != nil {
		t.Fatalf("ResolveBearerToken error: %v", err)
	}
	if principal.ActorType != ActorTypeIntegrationToken || principal.ActorID != "token-1" {
		t.Fatalf("principal = %#v, want integration token actor", principal)
	}
	if !repo.touched {
		t.Fatal("ResolveBearerToken did not touch token usage")
	}
}

func TestResolveBearerTokenReturnsUserSessionPrincipal(t *testing.T) {
	t.Parallel()

	repo := &fakeAuthRepo{
		sessionUser: CurrentUser{ID: "user-1", Email: "user@example.com"},
	}
	svc := NewAuthService(repo, NewPasswordHasher())

	principal, err := svc.ResolveBearerToken(context.Background(), "desktop-session-token")
	if err != nil {
		t.Fatalf("ResolveBearerToken error: %v", err)
	}
	if principal.ActorType != ActorTypeUserSession || principal.ActorID != "user-1" {
		t.Fatalf("principal = %#v, want user session actor", principal)
	}
	if !repo.sessionTouched {
		t.Fatal("ResolveBearerToken did not touch session usage")
	}
	if principal.SessionTokenHash != hashSessionToken("desktop-session-token") {
		t.Fatal("missing issuing session identity")
	}
	wire, err := json.Marshal(principal)
	if err != nil || strings.Contains(string(wire), principal.SessionTokenHash) {
		t.Fatal("internal session identity must not be serialized")
	}
}

func TestResolveBearerTokenRejectsUnknownToken(t *testing.T) {
	t.Parallel()

	svc := NewAuthService(&fakeAuthRepo{}, NewPasswordHasher())
	if _, err := svc.ResolveBearerToken(context.Background(), "unknown"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("ResolveBearerToken error = %v, want unauthenticated", err)
	}
}

func TestResolveBearerPrincipalParsesAuthorizationHeader(t *testing.T) {
	t.Parallel()

	repo := &fakeAuthRepo{
		principalRecord: IntegrationTokenPrincipalRecord{
			ID:     "token-1",
			UserID: "user-1",
			Email:  "user@example.com",
			Scopes: []string{ScopeArtifactsRead},
		},
	}
	svc := NewAuthService(repo, NewPasswordHasher())

	principal, err := ResolveBearerPrincipal(context.Background(), svc, "Bearer aladin_it_raw")
	if err != nil {
		t.Fatalf("ResolveBearerPrincipal error: %v", err)
	}
	if principal.ActorType != ActorTypeIntegrationToken {
		t.Fatalf("actor type = %q, want integration_token", principal.ActorType)
	}
}

func TestResolveBearerPrincipalRejectsMalformedAuthorizationHeader(t *testing.T) {
	t.Parallel()

	svc := NewAuthService(&fakeAuthRepo{}, NewPasswordHasher())
	for _, header := range []string{"", "Basic abc", "Bearer", "Bearer a b"} {
		if _, err := ResolveBearerPrincipal(context.Background(), svc, header); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("ResolveBearerPrincipal(%q) error = %v, want unauthenticated", header, err)
		}
	}
}

func TestRequireScopeHonorsIntegrationTokenScopes(t *testing.T) {
	t.Parallel()

	ctx := WithPrincipal(context.Background(), Principal{
		UserID:    "user-1",
		ActorType: ActorTypeIntegrationToken,
		ActorID:   "token-1",
		Scopes:    []string{ScopeArtifactsRead},
	})
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		t.Fatalf("RequireScope read error: %v", err)
	}
	if err := RequireScope(ctx, ScopeArtifactsWrite); !errors.Is(err, ErrForbidden) {
		t.Fatalf("RequireScope write error = %v, want forbidden", err)
	}
}

type contentTokenFixture struct {
	userID           string
	sessionTokenHash string
}

type fakeAuthRepo struct {
	created         IntegrationTokenRecord
	principalRecord IntegrationTokenPrincipalRecord
	sessionUser     CurrentUser
	touched         bool
	sessionTouched  bool
	contentTokens   map[string]contentTokenFixture
	sessions        map[string]AuthSessionRecord
	revokedSessions map[string]bool
}

func (r *fakeAuthRepo) CreateUser(context.Context, string, string) (CurrentUser, error) {
	return CurrentUser{}, nil
}

func (r *fakeAuthRepo) GetUserByEmail(context.Context, string) (AuthUserRecord, error) {
	return AuthUserRecord{}, ErrNotFound
}

func (r *fakeAuthRepo) GetUserBySessionTokenHash(_ context.Context, hash string, now time.Time) (CurrentUser, error) {
	if session, ok := r.sessions[hash]; ok && session.ExpiresAt.After(now) && !r.revokedSessions[hash] {
		return CurrentUser{ID: session.UserID, Email: "user@example.com"}, nil
	}
	if hash == hashSessionToken("desktop-session-token") && r.sessionUser.ID != "" {
		return r.sessionUser, nil
	}
	return CurrentUser{}, ErrUnauthenticated
}

func (r *fakeAuthRepo) CreateSession(_ context.Context, rec AuthSessionRecord) error {
	if r.sessions == nil {
		r.sessions = map[string]AuthSessionRecord{}
	}
	r.sessions[rec.TokenHash] = rec
	return nil
}

func (r *fakeAuthRepo) RevokeSession(_ context.Context, hash string) error {
	if r.revokedSessions == nil {
		r.revokedSessions = map[string]bool{}
	}
	r.revokedSessions[hash] = true
	return nil
}

func (r *fakeAuthRepo) TouchSession(context.Context, string) error {
	r.sessionTouched = true
	return nil
}

func (r *fakeAuthRepo) MarkLogin(context.Context, string) error {
	return nil
}

func (r *fakeAuthRepo) CreateIntegrationToken(_ context.Context, rec IntegrationTokenRecord) (IntegrationToken, error) {
	r.created = rec
	return IntegrationToken{
		ID:     "token-1",
		Name:   rec.Name,
		Scopes: rec.Scopes,
		Status: "active",
	}, nil
}

func (r *fakeAuthRepo) ListIntegrationTokens(context.Context, string) ([]IntegrationToken, error) {
	return nil, nil
}

func (r *fakeAuthRepo) RevokeIntegrationToken(context.Context, string, string) error {
	return nil
}

func (r *fakeAuthRepo) GetIntegrationTokenByHash(context.Context, string, time.Time) (IntegrationTokenPrincipalRecord, error) {
	if r.principalRecord.ID == "" {
		return IntegrationTokenPrincipalRecord{}, ErrUnauthenticated
	}
	return r.principalRecord, nil
}

func (r *fakeAuthRepo) TouchIntegrationToken(context.Context, string) error {
	r.touched = true
	return nil
}

func (r *fakeAuthRepo) CreateContentToken(ctx context.Context, tokenHash, userID, sessionTokenHash string, now time.Time) (time.Time, error) {
	user, err := r.GetUserBySessionTokenHash(ctx, sessionTokenHash, now)
	if err != nil || user.ID != userID {
		return time.Time{}, ErrUnauthenticated
	}
	if r.contentTokens == nil {
		r.contentTokens = map[string]contentTokenFixture{}
	}
	r.contentTokens[tokenHash] = contentTokenFixture{userID: userID, sessionTokenHash: sessionTokenHash}
	return r.sessions[sessionTokenHash].ExpiresAt, nil
}

func (r *fakeAuthRepo) GetContentTokenUser(ctx context.Context, tokenHash string, now time.Time) (CurrentUser, error) {
	rec, ok := r.contentTokens[tokenHash]
	if !ok {
		return CurrentUser{}, ErrUnauthenticated
	}
	user, err := r.GetUserBySessionTokenHash(ctx, rec.sessionTokenHash, now)
	if err != nil || user.ID != rec.userID {
		return CurrentUser{}, ErrUnauthenticated
	}
	return user, nil
}

func (r *fakeAuthRepo) DeleteExpiredContentTokens(ctx context.Context, now time.Time) error {
	for hash := range r.contentTokens {
		if _, err := r.GetContentTokenUser(ctx, hash, now); err != nil {
			delete(r.contentTokens, hash)
		}
	}
	return nil
}

func TestContentTokenSharesIssuingSessionLifetime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	repo := &fakeAuthRepo{}
	svc := NewAuthService(repo, NewPasswordHasher())
	svc.now = func() time.Time { return now }
	session, err := svc.createSession(context.Background(), CurrentUser{ID: "user-1"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	// Mint a week into the session: inherit its expiry, not another 30 days.
	now = now.Add(7 * 24 * time.Hour)
	principal, err := svc.ResolveBearerToken(context.Background(), session.Token)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithPrincipal(context.Background(), principal)
	content, err := svc.MintContentToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if content.Token == session.Token || content.ExpiresAt != session.ExpiresAt.Format(time.RFC3339) {
		t.Fatal("content credential must be distinct and expire with its issuing session")
	}
	now = now.Add(48 * time.Hour)
	resolved, err := svc.ResolveBearerToken(context.Background(), content.Token)
	if err != nil || resolved.ActorType != ActorTypeContentToken || len(resolved.Scopes) != 1 || resolved.Scopes[0] != ScopeContentRead || resolved.SessionTokenHash != "" {
		t.Fatalf("content credential past 12h = %+v, err=%v", resolved, err)
	}
	if _, err := svc.MintContentToken(WithPrincipal(context.Background(), resolved)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("content token minted a new credential: %v", err)
	}
	now = session.ExpiresAt
	if _, err := svc.ResolveBearerToken(context.Background(), content.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expired session still opens content: %v", err)
	}
	if _, err := svc.MintContentToken(ctx); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("session expiring after authentication still minted content: %v", err)
	}
}

func TestContentTokenLogoutRevokesOnlyIssuingSession(t *testing.T) {
	t.Parallel()
	repo := &fakeAuthRepo{}
	svc := NewAuthService(repo, NewPasswordHasher())
	ctx := context.Background()
	var sessions []AuthSession
	var tokens []ContentToken
	for i := 0; i < 2; i++ {
		session, err := svc.createSession(ctx, CurrentUser{ID: "user-1"}, "test")
		if err != nil {
			t.Fatal(err)
		}
		principal, err := svc.ResolveBearerToken(ctx, session.Token)
		if err != nil {
			t.Fatal(err)
		}
		content, err := svc.MintContentToken(WithPrincipal(ctx, principal))
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, session)
		tokens = append(tokens, content)
	}
	if err := svc.Logout(ctx, sessions[0].Token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ResolveBearerToken(ctx, tokens[0].Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("logged-out session still opens content: %v", err)
	}
	if _, err := svc.ResolveBearerToken(ctx, tokens[1].Token); err != nil {
		t.Fatalf("another session lost access: %v", err)
	}
}

func TestContentTokenMintRequiresIdentifiedUserSession(t *testing.T) {
	t.Parallel()
	svc := NewAuthService(&fakeAuthRepo{}, NewPasswordHasher())
	for _, actor := range []string{ActorTypeUserSession, ActorTypeContentToken, ActorTypeIntegrationToken} {
		ctx := WithPrincipal(context.Background(), Principal{UserID: "user-1", ActorType: actor})
		if _, err := svc.MintContentToken(ctx); err == nil {
			t.Fatalf("actor %s minted content without a login session", actor)
		}
	}
}
