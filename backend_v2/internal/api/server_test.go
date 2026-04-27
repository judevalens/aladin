package api

import (
	"aladin/backend_v2/internal/app"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	artifactservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	server := New(":0", &pgxpool.Pool{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got := body["service"]; got != "api" {
		t.Fatalf("service = %v, want api", got)
	}
}

func TestShutdownWithoutRun(t *testing.T) {
	t.Parallel()

	server := New(":0", &pgxpool.Pool{})
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}
}

func TestArtifactsCreate(t *testing.T) {
	t.Parallel()

	service := &fakeArtifactService{}
	server := NewWithDependencies(":0", app.StaticDependencies{ArtifactsSvc: service})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/artifacts/",
		strings.NewReader(`{"type":"note","title":"Rivian thesis","content":"Supply chain notes"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(service.created) != 1 {
		t.Fatalf("created calls = %d, want 1", len(service.created))
	}

	var body artifactservice.ArtifactResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Title != "Rivian thesis" {
		t.Fatalf("title = %q, want Rivian thesis", body.Title)
	}
}

func TestArtifactsList(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", app.StaticDependencies{
		ArtifactsSvc: &fakeArtifactService{
			list: []artifactservice.ArtifactResponse{
				{
					ID:        "artifact-1",
					Type:      "note",
					Title:     "Market memo",
					Content:   "memo",
					Metadata:  map[string]any{},
					CreatedAt: "2026-04-25T00:00:00Z",
					UpdatedAt: "2026-04-25T00:00:00Z",
				},
			},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/artifacts/", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body []artifactservice.ArtifactResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body) != 1 || body[0].ID != "artifact-1" {
		t.Fatalf("body = %#v, want artifact-1", body)
	}
}

func TestNotesCreateRoute(t *testing.T) {
	t.Parallel()

	service := &fakeArtifactService{}
	server := NewWithDependencies(":0", app.StaticDependencies{ArtifactsSvc: service})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/notes/",
		strings.NewReader(`{"label":"Quick note","content":"artifact-owned"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(service.noteCalls) != 1 || service.noteCalls[0].Content != "artifact-owned" {
		t.Fatalf("created = %#v, want one created note", service.noteCalls)
	}
}

func TestDocumentsListRoute(t *testing.T) {
	t.Parallel()

	server := NewWithDependencies(":0", app.StaticDependencies{
		ArtifactsSvc: &fakeArtifactService{
			documents: []artifactservice.DocumentRecord{{ID: "sha256:1", Filename: "memo.pdf", Ingested: true}},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/documents/", nil)
	rec := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body []artifactservice.DocumentRecord
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body) != 1 || body[0].Filename != "memo.pdf" {
		t.Fatalf("body = %#v, want memo.pdf", body)
	}
}

type fakeArtifactService struct {
	list      []artifactservice.ArtifactResponse
	created   []artifactservice.ArtifactPayload
	noteCalls []artifactservice.NoteRecord
	documents []artifactservice.DocumentRecord
}

func (f *fakeArtifactService) List(context.Context) ([]artifactservice.ArtifactResponse, error) {
	return f.list, nil
}

func (f *fakeArtifactService) Get(context.Context, string) (artifactservice.ArtifactResponse, error) {
	return artifactservice.ArtifactResponse{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) Create(_ context.Context, payload artifactservice.ArtifactPayload) (artifactservice.ArtifactResponse, error) {
	f.created = append(f.created, payload)
	return artifactservice.ArtifactResponse{
		ID:        "artifact-created",
		Type:      payload.Type,
		Title:     payload.Title,
		Content:   payload.Content,
		Summary:   payload.Summary,
		SourceURL: payload.SourceURL,
		Metadata:  map[string]any{},
		CreatedAt: "2026-04-25T00:00:00Z",
		UpdatedAt: "2026-04-25T00:00:00Z",
	}, nil
}

func (f *fakeArtifactService) Update(context.Context, string, artifactservice.ArtifactPatch) (artifactservice.ArtifactResponse, error) {
	return artifactservice.ArtifactResponse{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) Delete(context.Context, string) error {
	return artifactservice.ErrNotFound
}

func (f *fakeArtifactService) CreateNote(_ context.Context, label string, content string) (artifactservice.NoteRecord, error) {
	rec := artifactservice.NoteRecord{
		ID:        "note-1",
		Type:      "note",
		Label:     label,
		Content:   content,
		Metadata:  map[string]any{},
		Status:    "pending",
		CreatedAt: "2026-04-25T00:00:00Z",
	}
	f.noteCalls = append(f.noteCalls, rec)
	return rec, nil
}

func (f *fakeArtifactService) UpdateNote(context.Context, string, *string, *string) (artifactservice.NoteRecord, error) {
	return artifactservice.NoteRecord{}, artifactservice.ErrNotFound
}

func (f *fakeArtifactService) DeleteNote(context.Context, string) error {
	return nil
}

func (f *fakeArtifactService) RelatedNotes(context.Context, string, int) (artifactservice.RelatedNotesResponse, error) {
	return artifactservice.RelatedNotesResponse{Results: []artifactservice.RelatedRecord{}}, nil
}

func (f *fakeArtifactService) UploadDocument(context.Context, string, []byte) (artifactservice.DocumentRecord, error) {
	return artifactservice.DocumentRecord{}, nil
}

func (f *fakeArtifactService) ListDocuments(context.Context) ([]artifactservice.DocumentRecord, error) {
	return f.documents, nil
}

func (f *fakeArtifactService) DocumentFilePath(string) (string, error) {
	return "", artifactservice.ErrNotFound
}

func (f *fakeArtifactService) UploadAudio(context.Context, string, io.Reader) (artifactservice.AudioUploadRecord, error) {
	return artifactservice.AudioUploadRecord{}, nil
}

func (f *fakeArtifactService) AudioFilePath(string) (string, error) {
	return "", artifactservice.ErrNotFound
}
