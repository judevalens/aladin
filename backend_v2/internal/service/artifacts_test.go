package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestArtifactServiceCreateDefaultsAndTrims(t *testing.T) {
	t.Parallel()

	summary := "  useful memo  "
	sourceURL := "  https://example.com/rivian  "
	repo := &fakeArtifactRepository{}
	svc := NewArtifactService(repo, &fakeArtifactFiles{})

	rec, err := svc.Create(context.Background(), ArtifactPayload{
		Content:   "  Rivian supply chain memo  ",
		Summary:   &summary,
		SourceURL: &sourceURL,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if rec.ID == "" || !strings.HasPrefix(rec.ID, "artifact-") {
		t.Fatalf("id = %q, want artifact-*", rec.ID)
	}
	if len(repo.createdArtifacts) != 1 || repo.createdArtifacts[0].ID != rec.ID {
		t.Fatalf("created = %#v, want one created artifact", repo.createdArtifacts)
	}
}

func TestArtifactServiceCreateRejectsMissingTitleAndContent(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	_, err := svc.Create(context.Background(), ArtifactPayload{Type: "note"})
	if err == nil {
		t.Fatal("Create error = nil, want BadRequest")
	}
	var requestErr BadRequest
	if !errors.As(err, &requestErr) {
		t.Fatalf("Create error = %T, want BadRequest", err)
	}
}

func TestArtifactServiceEmptyIDsAreNotFound(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{}, &fakeArtifactFiles{})
	if _, err := svc.Get(context.Background(), " "); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get error = %v, want ErrNotFound", err)
	}
	if _, err := svc.Update(context.Background(), " ", ArtifactPatch{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update error = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(context.Background(), " "); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete error = %v, want ErrNotFound", err)
	}
}

func TestArtifactServiceRelatedNotesMissingNoteIsNotFound(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{noteExists: false}, &fakeArtifactFiles{})
	_, err := svc.RelatedNotes(context.Background(), "note-1", 5)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("RelatedNotes error = %v, want ErrNotFound", err)
	}
}

func TestArtifactServiceRelatedNotesNotEmbeddedReturnsMessage(t *testing.T) {
	t.Parallel()

	svc := NewArtifactService(&fakeArtifactRepository{noteExists: true}, &fakeArtifactFiles{})
	out, err := svc.RelatedNotes(context.Background(), "note-1", 5)
	if err != nil {
		t.Fatalf("RelatedNotes error: %v", err)
	}
	if out.Message != "Note not yet embedded" {
		t.Fatalf("message = %q, want not embedded", out.Message)
	}
	if len(out.Results) != 0 {
		t.Fatalf("results = %#v, want empty", out.Results)
	}
}

type fakeArtifactRepository struct {
	artifactByID     map[string]ArtifactResponse
	createdArtifacts []ArtifactResponse
	noteExists       bool
}

func (f *fakeArtifactRepository) ListArtifacts(context.Context) ([]ArtifactResponse, error) {
	return nil, nil
}
func (f *fakeArtifactRepository) GetArtifact(_ context.Context, id string) (ArtifactResponse, error) {
	if f.artifactByID == nil {
		return ArtifactResponse{}, ErrNotFound
	}
	rec, ok := f.artifactByID[id]
	if !ok {
		return ArtifactResponse{}, ErrNotFound
	}
	return rec, nil
}
func (f *fakeArtifactRepository) CreateArtifact(_ context.Context, rec ArtifactResponse) error {
	f.createdArtifacts = append(f.createdArtifacts, rec)
	return nil
}
func (f *fakeArtifactRepository) UpdateArtifact(context.Context, string, ArtifactPatch) error {
	return nil
}
func (f *fakeArtifactRepository) DeleteArtifact(context.Context, string) error { return nil }
func (f *fakeArtifactRepository) FindDocument(context.Context, string) (DocumentRecord, bool, error) {
	return DocumentRecord{}, false, nil
}
func (f *fakeArtifactRepository) CreateDocument(context.Context, DocumentRecord, int64) error {
	return nil
}
func (f *fakeArtifactRepository) ListDocuments(context.Context) ([]DocumentRecord, error) {
	return nil, nil
}
func (f *fakeArtifactRepository) CreateNote(context.Context, NoteRecord) error { return nil }
func (f *fakeArtifactRepository) NoteExists(context.Context, string) (bool, error) {
	return f.noteExists, nil
}
func (f *fakeArtifactRepository) UpdateNote(context.Context, string, *string, *string) error {
	return nil
}
func (f *fakeArtifactRepository) DeleteNote(context.Context, string) error { return nil }
func (f *fakeArtifactRepository) FetchNote(context.Context, string) (NoteRecord, error) {
	return NoteRecord{}, nil
}
func (f *fakeArtifactRepository) HasNoteEmbedding(context.Context, string) (bool, error) {
	return false, nil
}
func (f *fakeArtifactRepository) RelatedNotes(context.Context, string, int) ([]RelatedRecord, error) {
	return nil, nil
}

type fakeArtifactFiles struct{}

func (f *fakeArtifactFiles) SaveDocument(string, []byte) error   { return nil }
func (f *fakeArtifactFiles) DocumentPath(string) (string, error) { return "", ErrNotFound }
func (f *fakeArtifactFiles) SaveAudio(string, io.Reader) (string, string, error) {
	return "audio-1.webm", "audio/webm", nil
}
func (f *fakeArtifactFiles) AudioPath(string) (string, error) { return "", ErrNotFound }
