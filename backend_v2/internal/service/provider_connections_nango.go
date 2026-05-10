package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type NangoClient interface {
	CreateConnectSession(context.Context, NangoCreateConnectSessionRequest) (NangoConnectSession, error)
	ListConnections(context.Context, map[string]string) ([]NangoConnection, error)
	GetConnectionCredentials(context.Context, string, string, bool) (NangoConnectionCredentials, error)
	DeleteConnection(context.Context, string, string) error
}

type NangoCreateConnectSessionRequest struct {
	Tags                map[string]string `json:"tags"`
	AllowedIntegrations []string          `json:"allowed_integrations"`
}

type NangoConnectSession struct {
	Token       string     `json:"token"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	ConnectLink string     `json:"connect_link"`
}

type NangoConnection struct {
	ConnectionID      string         `json:"connection_id"`
	Provider          string         `json:"provider"`
	ProviderConfigKey string         `json:"provider_config_key"`
	Metadata          map[string]any `json:"metadata"`
	Tags              map[string]any `json:"tags"`
	Errors            []any          `json:"errors"`
}

type NangoConnectionCredentials struct {
	ConnectionID      string         `json:"connection_id"`
	Provider          string         `json:"provider"`
	ProviderConfigKey string         `json:"provider_config_key"`
	Credentials       map[string]any `json:"credentials"`
	Metadata          map[string]any `json:"metadata"`
	Tags              map[string]any `json:"tags"`
}

type NangoProviderConnectionBackend struct {
	client         NangoClient
	baseURL        string
	connectBaseURL string
	secretKey      string
}

func NewNangoProviderConnectionBackend(client NangoClient, baseURL, connectBaseURL, secretKey string) *NangoProviderConnectionBackend {
	return &NangoProviderConnectionBackend{
		client:         client,
		baseURL:        strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		connectBaseURL: strings.TrimRight(strings.TrimSpace(connectBaseURL), "/"),
		secretKey:      strings.TrimSpace(secretKey),
	}
}

func (b *NangoProviderConnectionBackend) Key() string { return ProviderConnectionBackendNango }

func (b *NangoProviderConnectionBackend) Available() bool {
	return b.client != nil && b.baseURL != "" && b.secretKey != ""
}

func (b *NangoProviderConnectionBackend) StartConnect(ctx context.Context, input ProviderConnectBackendInput) (ProviderConnectSession, error) {
	session, err := b.client.CreateConnectSession(ctx, NangoCreateConnectSessionRequest{
		Tags: map[string]string{
			"end_user_id":    input.Principal.UserID,
			"end_user_email": input.Principal.Email,
			"aladin_user_id": input.Principal.UserID,
		},
		AllowedIntegrations: []string{input.ProviderConfigKey},
	})
	if err != nil {
		return ProviderConnectSession{}, providerConnectionError("create Nango connect session", err)
	}
	connectLink := strings.TrimSpace(session.ConnectLink)
	if connectLink == "" && b.connectBaseURL != "" && session.Token != "" {
		connectLink = b.connectBaseURL + "?session_token=" + url.QueryEscape(session.Token)
	}
	return ProviderConnectSession{
		ConnectSessionToken: session.Token,
		ConnectLink:         connectLink,
		ExpiresAt:           session.ExpiresAt,
		NangoBaseURL:        b.baseURL,
		NangoConnectBaseURL: b.connectBaseURL,
	}, nil
}

func (b *NangoProviderConnectionBackend) ListConnections(ctx context.Context, principal ProviderConnectionPrincipal) ([]ProviderBackendConnection, error) {
	connections, err := b.client.ListConnections(ctx, map[string]string{"end_user_id": principal.UserID})
	if err != nil {
		return nil, providerConnectionError("list Nango connections", err)
	}
	out := make([]ProviderBackendConnection, 0, len(connections))
	for _, connection := range connections {
		out = append(out, ProviderBackendConnection{
			Provider:             connection.Provider,
			Backend:              ProviderConnectionBackendNango,
			ProviderConfigKey:    connection.ProviderConfigKey,
			ExternalConnectionID: connection.ConnectionID,
			GrantedScopes:        extractGrantedScopes(connection.Metadata),
			Metadata:             nangoConnectionMetadata(connection),
		})
	}
	return out, nil
}

func (b *NangoProviderConnectionBackend) Disconnect(ctx context.Context, connection ProviderConnection) error {
	if err := b.client.DeleteConnection(ctx, connection.ExternalConnectionID, connection.ProviderConfigKey); err != nil {
		return providerConnectionError("delete Nango connection", err)
	}
	return nil
}

func (b *NangoProviderConnectionBackend) GetConnectionCredentials(ctx context.Context, input ProviderCredentialRequest, connection ProviderConnection) (ProviderCredentials, error) {
	credentials, err := b.client.GetConnectionCredentials(ctx, connection.ExternalConnectionID, connection.ProviderConfigKey, input.ForceRefresh)
	if err != nil {
		return ProviderCredentials{}, providerConnectionError("get Nango connection credentials", err)
	}
	return ProviderCredentials{
		Provider:             connection.Provider,
		Backend:              connection.Backend,
		ProviderConfigKey:    connection.ProviderConfigKey,
		ExternalConnectionID: connection.ExternalConnectionID,
		Credentials:          nonNilMap(credentials.Credentials),
		Metadata:             nonNilMap(credentials.Metadata),
	}, nil
}

type HTTPNangoClient struct {
	baseURL    string
	secretKey  string
	httpClient *http.Client
}

func NewHTTPNangoClient(baseURL, secretKey string) *HTTPNangoClient {
	return &HTTPNangoClient{
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		secretKey: strings.TrimSpace(secretKey),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *HTTPNangoClient) CreateConnectSession(ctx context.Context, input NangoCreateConnectSessionRequest) (NangoConnectSession, error) {
	var response struct {
		Data NangoConnectSession `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/connect/sessions", nil, input, &response); err != nil {
		return NangoConnectSession{}, err
	}
	return response.Data, nil
}

func (c *HTTPNangoClient) ListConnections(ctx context.Context, tags map[string]string) ([]NangoConnection, error) {
	query := url.Values{}
	for key, value := range tags {
		query.Set("tags["+key+"]", value)
	}
	var response struct {
		Connections []NangoConnection `json:"connections"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/connections", query, nil, &response); err != nil {
		return nil, err
	}
	return response.Connections, nil
}

func (c *HTTPNangoClient) GetConnectionCredentials(ctx context.Context, connectionID string, providerConfigKey string, forceRefresh bool) (NangoConnectionCredentials, error) {
	query := url.Values{}
	query.Set("provider_config_key", providerConfigKey)
	if forceRefresh {
		query.Set("force_refresh", "true")
	}
	var response NangoConnectionCredentials
	if err := c.doJSON(ctx, http.MethodGet, "/connections/"+url.PathEscape(connectionID), query, nil, &response); err != nil {
		return NangoConnectionCredentials{}, err
	}
	return response, nil
}

func (c *HTTPNangoClient) DeleteConnection(ctx context.Context, connectionID string, providerConfigKey string) error {
	query := url.Values{}
	query.Set("provider_config_key", providerConfigKey)
	var response map[string]any
	return c.doJSON(ctx, http.MethodDelete, "/connections/"+url.PathEscape(connectionID), query, nil, &response)
}

func (c *HTTPNangoClient) doJSON(ctx context.Context, method string, path string, query url.Values, body any, out any) error {
	if c.baseURL == "" || c.secretKey == "" {
		return ErrProviderConnectionUnavailable
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	requestURL := c.baseURL + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("nango %s %s failed: %d %s", method, path, res.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func nangoConnectionMetadata(connection NangoConnection) map[string]any {
	metadata := nonNilMap(connection.Metadata)
	if len(connection.Tags) > 0 {
		metadata["tags"] = connection.Tags
	}
	if len(connection.Errors) > 0 {
		metadata["errors"] = connection.Errors
	}
	return metadata
}

func extractGrantedScopes(metadata map[string]any) []string {
	if metadata == nil {
		return []string{}
	}
	if raw, ok := metadata["granted_scopes"].([]any); ok {
		scopes := make([]string, 0, len(raw))
		for _, item := range raw {
			if scope, ok := item.(string); ok && strings.TrimSpace(scope) != "" {
				scopes = append(scopes, scope)
			}
		}
		return scopes
	}
	if raw, ok := metadata["granted_scopes"].([]string); ok {
		return raw
	}
	return []string{}
}
