package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

const (
	SessionCookieName = "aladin_session"
	sessionByteLength = 32
	sessionTTL        = 30 * 24 * time.Hour
)

var ErrUnauthenticated = errors.New("unauthenticated")
var ErrForbidden = errors.New("forbidden")

type authContextKey string

const principalContextKey authContextKey = "principal"

const (
	ActorTypeUserSession      = "user_session"
	ActorTypeIntegrationToken = "integration_token"
)

const (
	ScopeArtifactsRead  = "artifacts:read"
	ScopeArtifactsWrite = "artifacts:write"
	ScopeSourcesRead    = "sources:read"
	ScopeSourcesWrite   = "sources:write"
	ScopeInsightsRead   = "insights:read"
	ScopeAll            = "*"
)

type CurrentUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Principal struct {
	UserID    string
	ActorType string
	ActorID   string
	Email     string
	Scopes    []string
}

type AuthService interface {
	Register(context.Context, AuthCredentials, string) (AuthSession, error)
	Login(context.Context, AuthCredentials, string) (AuthSession, error)
	Logout(context.Context, string) error
	CurrentUser(context.Context, string) (CurrentUser, error)
	CreateIntegrationToken(context.Context, IntegrationTokenInput) (CreatedIntegrationToken, error)
	ListIntegrationTokens(context.Context) ([]IntegrationToken, error)
	RevokeIntegrationToken(context.Context, string) error
	ResolveBearerToken(context.Context, string) (Principal, error)
}

type AuthRepository interface {
	CreateUser(context.Context, string, string) (CurrentUser, error)
	GetUserByEmail(context.Context, string) (AuthUserRecord, error)
	GetUserBySessionTokenHash(context.Context, string, time.Time) (CurrentUser, error)
	CreateSession(context.Context, AuthSessionRecord) error
	RevokeSession(context.Context, string) error
	TouchSession(context.Context, string) error
	MarkLogin(context.Context, string) error
	CreateIntegrationToken(context.Context, IntegrationTokenRecord) (IntegrationToken, error)
	ListIntegrationTokens(context.Context, string) ([]IntegrationToken, error)
	RevokeIntegrationToken(context.Context, string, string) error
	GetIntegrationTokenByHash(context.Context, string, time.Time) (IntegrationTokenPrincipalRecord, error)
	TouchIntegrationToken(context.Context, string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(encoded string, password string) bool
}

type AuthCredentials struct {
	Email    string
	Password string
}

type AuthSession struct {
	User      CurrentUser
	Token     string
	ExpiresAt time.Time
}

type AuthUserRecord struct {
	ID           string
	Email        string
	PasswordHash string
}

type AuthSessionRecord struct {
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UserAgent string
}

type IntegrationTokenInput struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

type CreatedIntegrationToken struct {
	Token            string           `json:"token"`
	IntegrationToken IntegrationToken `json:"integrationToken"`
}

type IntegrationToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Scopes     []string   `json:"scopes"`
	Status     string     `json:"status"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

type IntegrationTokenRecord struct {
	UserID    string
	Name      string
	TokenHash string
	Scopes    []string
	ExpiresAt *time.Time
}

type IntegrationTokenPrincipalRecord struct {
	ID     string
	UserID string
	Email  string
	Scopes []string
}

type AuthServiceImpl struct {
	repo   AuthRepository
	hasher PasswordHasher
	now    func() time.Time
}

func NewAuthService(repo AuthRepository, hasher PasswordHasher) *AuthServiceImpl {
	return &AuthServiceImpl{
		repo:   repo,
		hasher: hasher,
		now:    time.Now,
	}
}

func (s *AuthServiceImpl) Register(ctx context.Context, input AuthCredentials, userAgent string) (AuthSession, error) {
	email, password, err := normalizeAuthCredentials(input)
	if err != nil {
		return AuthSession{}, err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return AuthSession{}, fmt.Errorf("hash password: %w", err)
	}
	user, err := s.repo.CreateUser(ctx, email, hash)
	if err != nil {
		return AuthSession{}, err
	}
	return s.createSession(ctx, user, userAgent)
}

func (s *AuthServiceImpl) Login(ctx context.Context, input AuthCredentials, userAgent string) (AuthSession, error) {
	email, password, err := normalizeAuthCredentials(input)
	if err != nil {
		return AuthSession{}, ErrUnauthenticated
	}
	record, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil || !s.hasher.Compare(record.PasswordHash, password) {
		return AuthSession{}, ErrUnauthenticated
	}
	user := CurrentUser{ID: record.ID, Email: record.Email}
	if err := s.repo.MarkLogin(ctx, user.ID); err != nil {
		return AuthSession{}, err
	}
	return s.createSession(ctx, user, userAgent)
}

func (s *AuthServiceImpl) Logout(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	return s.repo.RevokeSession(ctx, hashSessionToken(token))
}

func (s *AuthServiceImpl) CurrentUser(ctx context.Context, token string) (CurrentUser, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return CurrentUser{}, ErrUnauthenticated
	}
	tokenHash := hashSessionToken(token)
	user, err := s.repo.GetUserBySessionTokenHash(ctx, tokenHash, s.now().UTC())
	if err != nil {
		return CurrentUser{}, ErrUnauthenticated
	}
	_ = s.repo.TouchSession(ctx, tokenHash)
	return user, nil
}

func (s *AuthServiceImpl) CreateIntegrationToken(ctx context.Context, input IntegrationTokenInput) (CreatedIntegrationToken, error) {
	principal, err := RequireUserSession(ctx)
	if err != nil {
		return CreatedIntegrationToken{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CreatedIntegrationToken{}, BadRequest("name is required")
	}
	scopes, err := normalizeIntegrationTokenScopes(input.Scopes)
	if err != nil {
		return CreatedIntegrationToken{}, err
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(s.now().UTC()) {
		return CreatedIntegrationToken{}, BadRequest("expiresAt must be in the future")
	}
	raw, err := randomIntegrationToken()
	if err != nil {
		return CreatedIntegrationToken{}, err
	}
	rec, err := s.repo.CreateIntegrationToken(ctx, IntegrationTokenRecord{
		UserID:    principal.UserID,
		Name:      name,
		TokenHash: hashSessionToken(raw),
		Scopes:    scopes,
		ExpiresAt: input.ExpiresAt,
	})
	if err != nil {
		return CreatedIntegrationToken{}, err
	}
	return CreatedIntegrationToken{Token: raw, IntegrationToken: rec}, nil
}

func (s *AuthServiceImpl) ListIntegrationTokens(ctx context.Context) ([]IntegrationToken, error) {
	principal, err := RequireUserSession(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.ListIntegrationTokens(ctx, principal.UserID)
}

func (s *AuthServiceImpl) RevokeIntegrationToken(ctx context.Context, id string) error {
	principal, err := RequireUserSession(ctx)
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrNotFound
	}
	return s.repo.RevokeIntegrationToken(ctx, principal.UserID, id)
}

func (s *AuthServiceImpl) ResolveBearerToken(ctx context.Context, token string) (Principal, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Principal{}, ErrUnauthenticated
	}
	tokenHash := hashSessionToken(token)
	rec, err := s.repo.GetIntegrationTokenByHash(ctx, tokenHash, s.now().UTC())
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	_ = s.repo.TouchIntegrationToken(ctx, tokenHash)
	return Principal{
		UserID:    rec.UserID,
		ActorType: ActorTypeIntegrationToken,
		ActorID:   rec.ID,
		Email:     rec.Email,
		Scopes:    rec.Scopes,
	}, nil
}

func (s *AuthServiceImpl) createSession(ctx context.Context, user CurrentUser, userAgent string) (AuthSession, error) {
	token, err := randomToken()
	if err != nil {
		return AuthSession{}, err
	}
	expiresAt := s.now().UTC().Add(sessionTTL)
	if err := s.repo.CreateSession(ctx, AuthSessionRecord{
		UserID:    user.ID,
		TokenHash: hashSessionToken(token),
		ExpiresAt: expiresAt,
		UserAgent: userAgent,
	}); err != nil {
		return AuthSession{}, err
	}
	return AuthSession{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func normalizeAuthCredentials(input AuthCredentials) (string, string, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return "", "", BadRequest("valid email is required")
	}
	password := input.Password
	if len(password) < 8 {
		return "", "", BadRequest("password must be at least 8 characters")
	}
	return email, password, nil
}

func randomToken() (string, error) {
	bytes := make([]byte, sessionByteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("random session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func randomIntegrationToken() (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	return "aladin_it_" + token, nil
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func NewUserSessionPrincipal(user CurrentUser) Principal {
	return Principal{
		UserID:    user.ID,
		ActorType: ActorTypeUserSession,
		ActorID:   user.ID,
		Email:     user.Email,
	}
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(Principal)
	return principal, ok
}

func RequirePrincipal(ctx context.Context) (Principal, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok || strings.TrimSpace(principal.UserID) == "" {
		return Principal{}, ErrUnauthenticated
	}
	return principal, nil
}

func RequireUserSession(ctx context.Context) (Principal, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return Principal{}, err
	}
	if principal.ActorType != ActorTypeUserSession {
		return Principal{}, ErrForbidden
	}
	return principal, nil
}

func HasScope(ctx context.Context, scope string) bool {
	principal, ok := PrincipalFromContext(ctx)
	if !ok || strings.TrimSpace(principal.UserID) == "" {
		return false
	}
	if principal.ActorType == ActorTypeUserSession {
		return true
	}
	for _, candidate := range principal.Scopes {
		if candidate == ScopeAll || candidate == scope {
			return true
		}
	}
	return false
}

func RequireScope(ctx context.Context, scope string) error {
	if _, err := RequirePrincipal(ctx); err != nil {
		return err
	}
	if !HasScope(ctx, scope) {
		return ErrForbidden
	}
	return nil
}

func WithCurrentUser(ctx context.Context, user CurrentUser) context.Context {
	return WithPrincipal(ctx, NewUserSessionPrincipal(user))
}

func CurrentUserFromContext(ctx context.Context) (CurrentUser, bool) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok || strings.TrimSpace(principal.UserID) == "" {
		return CurrentUser{}, false
	}
	return CurrentUser{ID: principal.UserID, Email: principal.Email}, true
}

func normalizeIntegrationTokenScopes(input []string) ([]string, error) {
	allowed := map[string]bool{
		ScopeArtifactsRead:  true,
		ScopeArtifactsWrite: true,
		ScopeSourcesRead:    true,
		ScopeSourcesWrite:   true,
		ScopeInsightsRead:   true,
		ScopeAll:            true,
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(input))
	for _, scope := range input {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if !allowed[scope] {
			return nil, BadRequest("unsupported scope: " + scope)
		}
		if !seen[scope] {
			seen[scope] = true
			out = append(out, scope)
		}
	}
	if len(out) == 0 {
		return nil, BadRequest("at least one scope is required")
	}
	return out, nil
}
