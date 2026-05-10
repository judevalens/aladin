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

type authContextKey string

const currentUserContextKey authContextKey = "current_user"

type CurrentUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type AuthService interface {
	Register(context.Context, AuthCredentials, string) (AuthSession, error)
	Login(context.Context, AuthCredentials, string) (AuthSession, error)
	Logout(context.Context, string) error
	CurrentUser(context.Context, string) (CurrentUser, error)
}

type AuthRepository interface {
	CreateUser(context.Context, string, string) (CurrentUser, error)
	GetUserByEmail(context.Context, string) (AuthUserRecord, error)
	GetUserBySessionTokenHash(context.Context, string, time.Time) (CurrentUser, error)
	CreateSession(context.Context, AuthSessionRecord) error
	RevokeSession(context.Context, string) error
	TouchSession(context.Context, string) error
	MarkLogin(context.Context, string) error
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

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func WithCurrentUser(ctx context.Context, user CurrentUser) context.Context {
	return context.WithValue(ctx, currentUserContextKey, user)
}

func CurrentUserFromContext(ctx context.Context) (CurrentUser, bool) {
	user, ok := ctx.Value(currentUserContextKey).(CurrentUser)
	return user, ok
}
