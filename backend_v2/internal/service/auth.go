package service

import domainauth "aladin/backend_v2/internal/auth"

const (
	SessionCookieName = domainauth.SessionCookieName
	SessionHeaderName = domainauth.SessionHeaderName

	ActorTypeUserSession      = domainauth.ActorTypeUserSession
	ActorTypeIntegrationToken = domainauth.ActorTypeIntegrationToken
	ActorTypeContentToken     = domainauth.ActorTypeContentToken

	ScopeArtifactsRead  = domainauth.ScopeArtifactsRead
	ScopeArtifactsWrite = domainauth.ScopeArtifactsWrite
	ScopeSourcesRead    = domainauth.ScopeSourcesRead
	ScopeSourcesWrite   = domainauth.ScopeSourcesWrite
	ScopeInsightsRead   = domainauth.ScopeInsightsRead
	ScopeContentRead    = domainauth.ScopeContentRead
	ScopeAll            = domainauth.ScopeAll
)

var (
	ErrUnauthenticated = domainauth.ErrUnauthenticated
	ErrForbidden       = domainauth.ErrForbidden
)

type CurrentUser = domainauth.CurrentUser
type Principal = domainauth.Principal
type AuthService = domainauth.AuthService
type AuthRepository = domainauth.AuthRepository
type ContentToken = domainauth.ContentToken
type PasswordHasher = domainauth.PasswordHasher
type AuthCredentials = domainauth.AuthCredentials
type AuthSession = domainauth.AuthSession
type AuthUserRecord = domainauth.AuthUserRecord
type AuthSessionRecord = domainauth.AuthSessionRecord
type IntegrationTokenInput = domainauth.IntegrationTokenInput
type CreatedIntegrationToken = domainauth.CreatedIntegrationToken
type IntegrationToken = domainauth.IntegrationToken
type IntegrationTokenRecord = domainauth.IntegrationTokenRecord
type IntegrationTokenPrincipalRecord = domainauth.IntegrationTokenPrincipalRecord
type AuthServiceImpl = domainauth.AuthServiceImpl
type PBKDF2PasswordHasher = domainauth.PBKDF2PasswordHasher

var (
	NewAuthService          = domainauth.NewAuthService
	NewPasswordHasher       = domainauth.NewPasswordHasher
	NewUserSessionPrincipal = domainauth.NewUserSessionPrincipal
	WithPrincipal           = domainauth.WithPrincipal
	PrincipalFromContext    = domainauth.PrincipalFromContext
	RequirePrincipal        = domainauth.RequirePrincipal
	RequireUserSession      = domainauth.RequireUserSession
	HasScope                = domainauth.HasScope
	RequireScope            = domainauth.RequireScope
	ResolveBearerPrincipal  = domainauth.ResolveBearerPrincipal
	WithCurrentUser         = domainauth.WithCurrentUser
	CurrentUserFromContext  = domainauth.CurrentUserFromContext
)
