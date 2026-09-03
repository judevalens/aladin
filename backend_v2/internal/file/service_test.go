package file

import (
	"aladin/backend_v2/internal/artifact"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	coreservice "aladin/backend_v2/internal/service"
)

func TestFileServiceUpload(t *testing.T) {
	t.Parallel()

	repo := &fakeFileRepository{}
	store := &fakeFileStore{}
	service := NewFileService(repo, store)

	rec, err := service.Upload(testPrincipalContext(), FileUploadInput{Filename: "memo.txt"}, bytes.NewBufferString("hello"))
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if rec.ID == "" {
		t.Fatalf("record id empty")
	}
	if rec.URL != "/api/files/"+rec.ID+"/resource" {
		t.Fatalf("url = %q, want /api/files/%s/resource", rec.URL, rec.ID)
	}
	if repo.created == nil || repo.created.StorageKey != "file/file-blob.txt" {
		t.Fatalf("created = %#v, want saved file record", repo.created)
	}
}

func TestFileServiceUploadRemovesBytesWhenRecordWriteFails(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("database unavailable")
	store := &fakeFileStore{}
	service := NewFileService(&fakeFileRepository{createErr: wantErr}, store)

	if _, err := service.Upload(testPrincipalContext(), FileUploadInput{Filename: "memo.txt"}, bytes.NewBufferString("hello")); !errors.Is(err, wantErr) {
		t.Fatalf("Upload error = %v, want %v", err, wantErr)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "file/file-blob.txt" {
		t.Fatalf("deleted = %#v, want stored resource compensation", store.deleted)
	}
}

func TestFileServiceResource(t *testing.T) {
	t.Parallel()

	service := NewFileService(
		&fakeFileRepository{
			record: FileRecord{ID: "file-1", StorageKey: "file/file-blob.txt"},
		},
		&fakeFileStore{path: "/tmp/file-blob.txt"},
	)

	resource, err := service.Resource(testPrincipalContext(), "file-1")
	if err != nil {
		t.Fatalf("Resource error: %v", err)
	}
	if resource.Path != "/tmp/file-blob.txt" {
		t.Fatalf("path = %q, want /tmp/file-blob.txt", resource.Path)
	}
}

func TestFileServiceReadOnlyTokenCannotUpload(t *testing.T) {
	t.Parallel()

	service := NewFileService(
		&fakeFileRepository{
			record: FileRecord{ID: "file-1", StorageKey: "file/file-blob.txt"},
		},
		&fakeFileStore{path: "/tmp/file-blob.txt"},
	)
	ctx := testIntegrationPrincipalContext(coreservice.ScopeArtifactsRead)

	if _, err := service.Resource(ctx, "file-1"); err != nil {
		t.Fatalf("Resource read-only error = %v, want nil", err)
	}
	if _, err := service.Upload(ctx, FileUploadInput{Filename: "memo.txt"}, bytes.NewBufferString("hello")); !errors.Is(err, coreservice.ErrForbidden) {
		t.Fatalf("Upload read-only error = %v, want ErrForbidden", err)
	}
}

type fakeFileRepository struct {
	record    FileRecord
	created   *FileRecord
	createErr error
}

func (f *fakeFileRepository) CreateFile(_ context.Context, rec FileRecord) error {
	if f.createErr != nil {
		return f.createErr
	}
	copyRec := rec
	f.created = &copyRec
	return nil
}

func (f *fakeFileRepository) GetFile(_ context.Context, _ string) (FileRecord, error) {
	if f.record.ID == "" {
		return FileRecord{}, coreservice.ErrNotFound
	}
	return f.record, nil
}

type fakeFileStore struct {
	path    string
	deleted []string
}

func (f *fakeFileStore) SaveResource(_ string, _ string, _ string, _ io.Reader) (artifact.StoredArtifactResource, error) {
	return artifact.StoredArtifactResource{
		StorageKey: "file/file-blob.txt",
	}, nil
}

func (f *fakeFileStore) ResourcePath(_ string) (string, error) {
	if f.path == "" {
		return "", coreservice.ErrNotFound
	}
	return f.path, nil
}

func testPrincipalContext() context.Context {
	return testIntegrationPrincipalContext(coreservice.ScopeArtifactsRead, coreservice.ScopeArtifactsWrite)
}

func testIntegrationPrincipalContext(scopes ...string) context.Context {
	return coreservice.WithPrincipal(context.Background(), coreservice.Principal{
		UserID:    "user-1",
		ActorType: coreservice.ActorTypeIntegrationToken,
		ActorID:   "test",
		Scopes:    scopes,
	})
}

func (f *fakeFileStore) DeleteResource(key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}
