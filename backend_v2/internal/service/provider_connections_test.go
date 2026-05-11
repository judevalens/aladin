package service

import (
	"context"
	"errors"
	"testing"
)

func TestProviderConnectionListProvidersHidesUnavailableDetails(t *testing.T) {
	t.Parallel()

	svc := NewProviderConnectionService(
		&fakeProviderConnectionRepo{},
		[]ProviderDefinition{
			{Provider: "google", Label: "Google", Backend: "nango", ProviderConfigKey: "google-dev"},
			{Provider: "slack", Label: "Slack", Backend: "nango", ProviderConfigKey: ""},
		},
		[]ProviderConnectionBackend{&fakeProviderConnectionBackend{available: true}},
	)

	providers, err := svc.ListProviders(testProviderConnectionPrincipalContext())
	if err != nil {
		t.Fatalf("ListProviders error = %v", err)
	}
	if len(providers) != 2 {
		t.Fatalf("providers len = %d, want 2", len(providers))
	}
	if !providers[0].Available {
		t.Fatalf("google available = false, want true")
	}
	if providers[1].Available {
		t.Fatalf("slack available = true, want false without provider config key")
	}
}

func TestProviderConnectionStartConnectRequiresPrincipal(t *testing.T) {
	t.Parallel()

	svc := NewProviderConnectionService(&fakeProviderConnectionRepo{}, nil, nil)
	if _, err := svc.StartConnect(context.Background(), StartProviderConnectInput{Provider: "google"}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("StartConnect error = %v, want ErrUnauthenticated", err)
	}
}

func TestProviderConnectionStartConnectSendsTagsAndAllowedIntegration(t *testing.T) {
	t.Parallel()

	backend := &fakeProviderConnectionBackend{
		available: true,
		session: ProviderConnectSession{
			ConnectSessionToken: "session-token",
		},
	}
	svc := NewProviderConnectionService(
		&fakeProviderConnectionRepo{},
		[]ProviderDefinition{{Provider: "google", Label: "Google", Backend: "nango", ProviderConfigKey: "google-dev"}},
		[]ProviderConnectionBackend{backend},
	)

	session, err := svc.StartConnect(testProviderConnectionPrincipalContext(), StartProviderConnectInput{Provider: "google"})
	if err != nil {
		t.Fatalf("StartConnect error = %v", err)
	}
	if session.ConnectSessionToken != "session-token" {
		t.Fatalf("session token = %q, want session-token", session.ConnectSessionToken)
	}
	if backend.connectInput.Principal.UserID != "user-1" {
		t.Fatalf("connect user id = %q, want user-1", backend.connectInput.Principal.UserID)
	}
	if backend.connectInput.ProviderConfigKey != "google-dev" {
		t.Fatalf("provider config key = %q, want google-dev", backend.connectInput.ProviderConfigKey)
	}
}

func TestProviderConnectionStartConnectRejectsUnavailableBackend(t *testing.T) {
	t.Parallel()

	svc := NewProviderConnectionService(
		&fakeProviderConnectionRepo{},
		[]ProviderDefinition{{Provider: "google", Label: "Google", Backend: "nango", ProviderConfigKey: "google-dev"}},
		[]ProviderConnectionBackend{&fakeProviderConnectionBackend{available: false}},
	)

	_, err := svc.StartConnect(testProviderConnectionPrincipalContext(), StartProviderConnectInput{Provider: "google"})
	var requestErr BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("StartConnect error = %v, want BadRequest", err)
	}
}

func TestProviderConnectionSyncIsUserScopedAndIdempotent(t *testing.T) {
	t.Parallel()

	repo := &fakeProviderConnectionRepo{}
	backend := &fakeProviderConnectionBackend{
		available: true,
		connections: []ProviderBackendConnection{
			{
				Provider:             "google",
				Backend:              "nango",
				ProviderConfigKey:    "google-dev",
				ExternalConnectionID: "nango-user-1-google",
				Metadata:             map[string]any{"email": "admin@email.com"},
			},
		},
	}
	svc := NewProviderConnectionService(
		repo,
		[]ProviderDefinition{{Provider: "google", Label: "Google", Backend: "nango", ProviderConfigKey: "google-dev"}},
		[]ProviderConnectionBackend{backend},
	)

	if _, err := svc.SyncConnections(testProviderConnectionPrincipalContext(), SyncProviderConnectionsInput{}); err != nil {
		t.Fatalf("SyncConnections first error = %v", err)
	}
	if _, err := svc.SyncConnections(testProviderConnectionPrincipalContext(), SyncProviderConnectionsInput{}); err != nil {
		t.Fatalf("SyncConnections second error = %v", err)
	}
	if len(repo.connections) != 1 {
		t.Fatalf("repo connections len = %d, want 1", len(repo.connections))
	}
	if backend.listPrincipal.UserID != "user-1" {
		t.Fatalf("list principal = %q, want user-1", backend.listPrincipal.UserID)
	}
}

func TestProviderConnectionDisconnectMarksInactive(t *testing.T) {
	t.Parallel()

	repo := &fakeProviderConnectionRepo{
		connections: map[string]ProviderConnection{
			"conn-1": {
				ID:                   "conn-1",
				UserID:               "user-1",
				Provider:             "google",
				Backend:              "nango",
				ProviderConfigKey:    "google-dev",
				ExternalConnectionID: "nango-user-1-google",
				Status:               "active",
			},
		},
	}
	backend := &fakeProviderConnectionBackend{available: true}
	svc := NewProviderConnectionService(
		repo,
		[]ProviderDefinition{{Provider: "google", Label: "Google", Backend: "nango", ProviderConfigKey: "google-dev"}},
		[]ProviderConnectionBackend{backend},
	)

	if err := svc.Disconnect(testProviderConnectionPrincipalContext(), "conn-1"); err != nil {
		t.Fatalf("Disconnect error = %v", err)
	}
	if repo.connections["conn-1"].Status != "disconnected" {
		t.Fatalf("status = %q, want disconnected", repo.connections["conn-1"].Status)
	}
	if !backend.disconnected {
		t.Fatal("backend disconnect not called")
	}
	if repo.disableStreamsForConnectionID != "conn-1" {
		t.Fatalf("disabled streams for = %q, want conn-1", repo.disableStreamsForConnectionID)
	}
}

func testProviderConnectionPrincipalContext() context.Context {
	return WithPrincipal(context.Background(), Principal{
		UserID:    "user-1",
		ActorType: ActorTypeUserSession,
		ActorID:   "user-1",
		Email:     "admin@email.com",
	})
}

type fakeProviderConnectionRepo struct {
	connections                   map[string]ProviderConnection
	disableStreamsForConnectionID string
}

func (r *fakeProviderConnectionRepo) ensure() {
	if r.connections == nil {
		r.connections = map[string]ProviderConnection{}
	}
}

func (r *fakeProviderConnectionRepo) ListProviderConnections(_ context.Context, userID string) ([]ProviderConnection, error) {
	r.ensure()
	out := []ProviderConnection{}
	for _, connection := range r.connections {
		if connection.UserID == userID && connection.Status == "active" {
			out = append(out, connection)
		}
	}
	return out, nil
}

func (r *fakeProviderConnectionRepo) UpsertProviderConnection(_ context.Context, userID string, input ProviderConnection) (ProviderConnection, error) {
	r.ensure()
	for id, existing := range r.connections {
		if existing.UserID == userID &&
			existing.Backend == input.Backend &&
			existing.ProviderConfigKey == input.ProviderConfigKey &&
			existing.ExternalConnectionID == input.ExternalConnectionID {
			input.ID = id
			input.UserID = userID
			input.Status = "active"
			r.connections[id] = input
			return input, nil
		}
	}
	input.ID = "conn-1"
	input.UserID = userID
	input.Status = "active"
	r.connections[input.ID] = input
	return input, nil
}

func (r *fakeProviderConnectionRepo) DisconnectProviderConnection(_ context.Context, userID string, id string) (ProviderConnection, error) {
	r.ensure()
	connection, ok := r.connections[id]
	if !ok || connection.UserID != userID || connection.Status != "active" {
		return ProviderConnection{}, ErrNotFound
	}
	connection.Status = "disconnected"
	r.connections[id] = connection
	return connection, nil
}

func (r *fakeProviderConnectionRepo) DisableProviderStreamsForConnection(_ context.Context, _ string, connectionID string) error {
	r.disableStreamsForConnectionID = connectionID
	return nil
}

type fakeProviderConnectionBackend struct {
	available     bool
	session       ProviderConnectSession
	connectInput  ProviderConnectBackendInput
	listPrincipal ProviderConnectionPrincipal
	connections   []ProviderBackendConnection
	disconnected  bool
}

func (b *fakeProviderConnectionBackend) Key() string { return "nango" }

func (b *fakeProviderConnectionBackend) Available() bool { return b.available }

func (b *fakeProviderConnectionBackend) StartConnect(_ context.Context, input ProviderConnectBackendInput) (ProviderConnectSession, error) {
	b.connectInput = input
	return b.session, nil
}

func (b *fakeProviderConnectionBackend) ListConnections(_ context.Context, principal ProviderConnectionPrincipal) ([]ProviderBackendConnection, error) {
	b.listPrincipal = principal
	return b.connections, nil
}

func (b *fakeProviderConnectionBackend) Disconnect(_ context.Context, _ ProviderConnection) error {
	b.disconnected = true
	return nil
}

func (b *fakeProviderConnectionBackend) GetConnectionCredentials(_ context.Context, _ ProviderCredentialRequest, connection ProviderConnection) (ProviderCredentials, error) {
	return ProviderCredentials{
		Provider:             connection.Provider,
		Backend:              connection.Backend,
		ProviderConfigKey:    connection.ProviderConfigKey,
		ExternalConnectionID: connection.ExternalConnectionID,
		Credentials:          map[string]any{"type": "OAUTH2"},
	}, nil
}
