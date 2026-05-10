package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ProviderConnectionBackendNango = "nango"
	ProviderGoogle                 = "google"
)

type ProviderConnectionService interface {
	ListProviders(context.Context) ([]ProviderDescriptor, error)
	StartConnect(context.Context, StartProviderConnectInput) (ProviderConnectSession, error)
	SyncConnections(context.Context, SyncProviderConnectionsInput) ([]ProviderConnection, error)
	ListConnections(context.Context) ([]ProviderConnection, error)
	Disconnect(context.Context, string) error
	GetConnectionCredentials(context.Context, ProviderCredentialRequest) (ProviderCredentials, error)
}

type ProviderConnectionRepository interface {
	ListProviderConnections(context.Context, string) ([]ProviderConnection, error)
	UpsertProviderConnection(context.Context, string, ProviderConnection) (ProviderConnection, error)
	DisconnectProviderConnection(context.Context, string, string) (ProviderConnection, error)
	DisableProviderStreamsForConnection(context.Context, string, string) error
}

type ProviderConnectionBackend interface {
	Key() string
	Available() bool
	StartConnect(context.Context, ProviderConnectBackendInput) (ProviderConnectSession, error)
	ListConnections(context.Context, ProviderConnectionPrincipal) ([]ProviderBackendConnection, error)
	Disconnect(context.Context, ProviderConnection) error
	GetConnectionCredentials(context.Context, ProviderCredentialRequest, ProviderConnection) (ProviderCredentials, error)
}

type ProviderDescriptor struct {
	Provider          string   `json:"provider"`
	Label             string   `json:"label"`
	Backend           string   `json:"backend"`
	ProviderConfigKey string   `json:"providerConfigKey"`
	Available         bool     `json:"available"`
	Connected         bool     `json:"connected"`
	ConnectionID      *string  `json:"connectionId,omitempty"`
	GrantedScopes     []string `json:"grantedScopes"`
}

type ProviderDefinition struct {
	Provider          string
	Label             string
	Backend           string
	ProviderConfigKey string
}

type StartProviderConnectInput struct {
	Provider string
}

type SyncProviderConnectionsInput struct{}

type ProviderConnectSession struct {
	ConnectSessionToken string     `json:"connectSessionToken"`
	ConnectLink         string     `json:"connectLink"`
	ExpiresAt           *time.Time `json:"expiresAt,omitempty"`
	NangoBaseURL        string     `json:"nangoBaseUrl"`
	NangoConnectBaseURL string     `json:"nangoConnectBaseUrl"`
}

type ProviderConnection struct {
	ID                   string         `json:"id"`
	UserID               string         `json:"-"`
	Provider             string         `json:"provider"`
	Backend              string         `json:"backend"`
	ProviderConfigKey    string         `json:"providerConfigKey"`
	ExternalConnectionID string         `json:"externalConnectionId"`
	Status               string         `json:"status"`
	GrantedScopes        []string       `json:"grantedScopes"`
	Metadata             map[string]any `json:"metadata"`
	CreatedAt            string         `json:"createdAt"`
	UpdatedAt            string         `json:"updatedAt"`
	DisconnectedAt       *string        `json:"disconnectedAt,omitempty"`
}

type ProviderBackendConnection struct {
	Provider             string
	Backend              string
	ProviderConfigKey    string
	ExternalConnectionID string
	GrantedScopes        []string
	Metadata             map[string]any
}

type ProviderCredentialRequest struct {
	ConnectionID string
	ForceRefresh bool
}

type ProviderCredentials struct {
	Provider             string         `json:"provider"`
	Backend              string         `json:"backend"`
	ProviderConfigKey    string         `json:"providerConfigKey"`
	ExternalConnectionID string         `json:"externalConnectionId"`
	Credentials          map[string]any `json:"credentials"`
	Metadata             map[string]any `json:"metadata"`
}

type ProviderConnectionPrincipal struct {
	UserID string
	Email  string
}

type ProviderConnectBackendInput struct {
	Principal         ProviderConnectionPrincipal
	Provider          string
	ProviderConfigKey string
}

type DefaultProviderConnectionService struct {
	repo        ProviderConnectionRepository
	definitions []ProviderDefinition
	backends    map[string]ProviderConnectionBackend
}

func NewProviderConnectionService(
	repo ProviderConnectionRepository,
	definitions []ProviderDefinition,
	backends []ProviderConnectionBackend,
) *DefaultProviderConnectionService {
	backendByKey := make(map[string]ProviderConnectionBackend, len(backends))
	for _, backend := range backends {
		if backend != nil {
			backendByKey[backend.Key()] = backend
		}
	}
	return &DefaultProviderConnectionService{repo: repo, definitions: definitions, backends: backendByKey}
}

func (s *DefaultProviderConnectionService) ListProviders(ctx context.Context) ([]ProviderDescriptor, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	connections, err := s.repo.ListProviderConnections(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	activeByProvider := map[string]ProviderConnection{}
	for _, connection := range connections {
		if connection.Status == "active" {
			activeByProvider[connection.Provider] = connection
		}
	}

	out := make([]ProviderDescriptor, 0, len(s.definitions))
	for _, def := range s.definitions {
		backend := s.backends[def.Backend]
		descriptor := ProviderDescriptor{
			Provider:          def.Provider,
			Label:             def.Label,
			Backend:           def.Backend,
			ProviderConfigKey: def.ProviderConfigKey,
			Available:         strings.TrimSpace(def.ProviderConfigKey) != "" && backend != nil && backend.Available(),
			GrantedScopes:     []string{},
		}
		if connection, ok := activeByProvider[def.Provider]; ok {
			id := connection.ID
			descriptor.Connected = true
			descriptor.ConnectionID = &id
			descriptor.GrantedScopes = connection.GrantedScopes
		}
		out = append(out, descriptor)
	}
	return out, nil
}

func (s *DefaultProviderConnectionService) StartConnect(ctx context.Context, input StartProviderConnectInput) (ProviderConnectSession, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return ProviderConnectSession{}, ErrUnauthenticated
	}
	def, backend, err := s.resolveProvider(input.Provider)
	if err != nil {
		return ProviderConnectSession{}, err
	}
	if strings.TrimSpace(def.ProviderConfigKey) == "" || !backend.Available() {
		return ProviderConnectSession{}, BadRequest("provider is not available")
	}
	return backend.StartConnect(ctx, ProviderConnectBackendInput{
		Principal: ProviderConnectionPrincipal{
			UserID: principal.UserID,
			Email:  principal.Email,
		},
		Provider:          def.Provider,
		ProviderConfigKey: def.ProviderConfigKey,
	})
}

func (s *DefaultProviderConnectionService) SyncConnections(ctx context.Context, _ SyncProviderConnectionsInput) ([]ProviderConnection, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	current := ProviderConnectionPrincipal{UserID: principal.UserID, Email: principal.Email}

	defByConfigKey := map[string]ProviderDefinition{}
	for _, def := range s.definitions {
		if strings.TrimSpace(def.ProviderConfigKey) != "" {
			defByConfigKey[def.ProviderConfigKey] = def
		}
	}

	out := []ProviderConnection{}
	for _, backend := range s.backends {
		if backend == nil || !backend.Available() {
			continue
		}
		remoteConnections, err := backend.ListConnections(ctx, current)
		if err != nil {
			return nil, err
		}
		for _, remote := range remoteConnections {
			def, ok := defByConfigKey[remote.ProviderConfigKey]
			if !ok {
				continue
			}
			saved, err := s.repo.UpsertProviderConnection(ctx, principal.UserID, ProviderConnection{
				Provider:             fallbackString(remote.Provider, def.Provider),
				Backend:              fallbackString(remote.Backend, def.Backend),
				ProviderConfigKey:    remote.ProviderConfigKey,
				ExternalConnectionID: remote.ExternalConnectionID,
				Status:               "active",
				GrantedScopes:        remote.GrantedScopes,
				Metadata:             nonNilMap(remote.Metadata),
			})
			if err != nil {
				return nil, err
			}
			out = append(out, saved)
		}
	}
	return out, nil
}

func (s *DefaultProviderConnectionService) ListConnections(ctx context.Context) ([]ProviderConnection, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	return s.repo.ListProviderConnections(ctx, principal.UserID)
}

func (s *DefaultProviderConnectionService) Disconnect(ctx context.Context, connectionID string) error {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return ErrUnauthenticated
	}
	connections, err := s.repo.ListProviderConnections(ctx, principal.UserID)
	if err != nil {
		return err
	}
	var connection ProviderConnection
	found := false
	for _, candidate := range connections {
		if candidate.ID == connectionID && candidate.Status == "active" {
			connection = candidate
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	if backend := s.backends[connection.Backend]; backend != nil && backend.Available() {
		if err := backend.Disconnect(ctx, connection); err != nil {
			return err
		}
	}
	if _, err := s.repo.DisconnectProviderConnection(ctx, principal.UserID, connectionID); err != nil {
		return err
	}
	return s.repo.DisableProviderStreamsForConnection(ctx, principal.UserID, connectionID)
}

func (s *DefaultProviderConnectionService) GetConnectionCredentials(ctx context.Context, input ProviderCredentialRequest) (ProviderCredentials, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return ProviderCredentials{}, ErrUnauthenticated
	}
	connections, err := s.repo.ListProviderConnections(ctx, principal.UserID)
	if err != nil {
		return ProviderCredentials{}, err
	}
	for _, connection := range connections {
		if connection.ID == input.ConnectionID && connection.Status == "active" {
			backend := s.backends[connection.Backend]
			if backend == nil || !backend.Available() {
				return ProviderCredentials{}, BadRequest("provider backend is not available")
			}
			return backend.GetConnectionCredentials(ctx, input, connection)
		}
	}
	return ProviderCredentials{}, ErrNotFound
}

func (s *DefaultProviderConnectionService) resolveProvider(provider string) (ProviderDefinition, ProviderConnectionBackend, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	for _, def := range s.definitions {
		if def.Provider == provider {
			backend := s.backends[def.Backend]
			if backend == nil {
				return ProviderDefinition{}, nil, BadRequest("provider backend is not available")
			}
			return def, backend, nil
		}
	}
	return ProviderDefinition{}, nil, ErrNotFound
}

func nonNilMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

var ErrProviderConnectionUnavailable = errors.New("provider connection unavailable")

func providerConnectionError(message string, err error) error {
	if err == nil {
		return fmt.Errorf("%w: %s", ErrProviderConnectionUnavailable, message)
	}
	return fmt.Errorf("%w: %s: %w", ErrProviderConnectionUnavailable, message, err)
}
