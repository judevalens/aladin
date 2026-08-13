package service

import (
	"context"
	"errors"
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
	userID    string
	expiresAt time.Time
}

type fakeAuthRepo struct {
	created         IntegrationTokenRecord
	principalRecord IntegrationTokenPrincipalRecord
	sessionUser     CurrentUser
	touched         bool
	sessionTouched  bool
	contentTokens   map[string]contentTokenFixture
}

func (r *fakeAuthRepo) CreateUser(context.Context, string, string) (CurrentUser, error) {
	return CurrentUser{}, nil
}

func (r *fakeAuthRepo) GetUserByEmail(context.Context, string) (AuthUserRecord, error) {
	return AuthUserRecord{}, ErrNotFound
}

func (r *fakeAuthRepo) GetUserBySessionTokenHash(context.Context, string, time.Time) (CurrentUser, error) {
	if r.sessionUser.ID == "" {
		return CurrentUser{}, ErrUnauthenticated
	}
	return r.sessionUser, nil
}

func (r *fakeAuthRepo) CreateSession(context.Context, AuthSessionRecord) error {
	return nil
}

func (r *fakeAuthRepo) RevokeSession(context.Context, string) error {
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

func (r *fakeAuthRepo) CreateContentToken(_ context.Context, tokenHash, userID string, expiresAt time.Time) error {
	if r.contentTokens == nil {
		r.contentTokens = map[string]contentTokenFixture{}
	}
	r.contentTokens[tokenHash] = contentTokenFixture{userID: userID, expiresAt: expiresAt}
	return nil
}

func (r *fakeAuthRepo) GetContentTokenUser(_ context.Context, tokenHash string, now time.Time) (CurrentUser, error) {
	rec, ok := r.contentTokens[tokenHash]
	if !ok || !rec.expiresAt.After(now) {
		return CurrentUser{}, ErrUnauthenticated
	}
	return CurrentUser{ID: rec.userID, Email: "content@example.com"}, nil
}

func (r *fakeAuthRepo) DeleteExpiredContentTokens(_ context.Context, now time.Time) error {
	for hash, rec := range r.contentTokens {
		if !rec.expiresAt.After(now) {
			delete(r.contentTokens, hash)
		}
	}
	return nil
}
