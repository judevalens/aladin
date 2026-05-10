package service

import (
	"bytes"
	"context"
	"io"
	"testing"
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

type fakeFileRepository struct {
	record  FileRecord
	created *FileRecord
}

func (f *fakeFileRepository) CreateFile(_ context.Context, rec FileRecord) error {
	copyRec := rec
	f.created = &copyRec
	return nil
}

func (f *fakeFileRepository) GetFile(_ context.Context, _ string) (FileRecord, error) {
	if f.record.ID == "" {
		return FileRecord{}, ErrNotFound
	}
	return f.record, nil
}

type fakeFileStore struct {
	path string
}

func (f *fakeFileStore) SaveResource(_ string, _ string, _ io.Reader) (StoredArtifactResource, error) {
	return StoredArtifactResource{
		StorageKey: "file/file-blob.txt",
	}, nil
}

func (f *fakeFileStore) ResourcePath(_ string) (string, error) {
	if f.path == "" {
		return "", ErrNotFound
	}
	return f.path, nil
}
