package repo

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	coreservice "aladin/backend_v2/internal/service"
)

type FilesystemArtifactStore struct {
	uploadDir string
	audioDir  string
}

func NewFilesystemArtifactStore(uploadDir string, audioDir string) *FilesystemArtifactStore {
	return &FilesystemArtifactStore{uploadDir: uploadDir, audioDir: audioDir}
}

func (s *FilesystemArtifactStore) SaveResource(kind string, filename string, contentType string, body io.Reader) (coreservice.StoredArtifactResource, error) {
	baseDir := s.baseDir(kind)
	if baseDir == "" {
		return coreservice.StoredArtifactResource{}, fmt.Errorf("unsupported artifact resource kind: %s", kind)
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return coreservice.StoredArtifactResource{}, err
	}
	ext := strings.ToLower(filepath.Ext(filepath.Base(filename)))
	storageKey := kind + "/" + coreservice.NewID(kind+"-", ext)
	path := filepath.Join(baseDir, filepath.Base(strings.TrimPrefix(storageKey, kind+"/")))
	out, err := os.Create(path)
	if err != nil {
		return coreservice.StoredArtifactResource{}, err
	}
	defer out.Close()
	size, err := io.Copy(out, body)
	if err != nil {
		return coreservice.StoredArtifactResource{}, err
	}
	// Trust the uploaded content-type (set from the browser's blob type) — the
	// client knows the real codec. Fall back to the file extension only when the
	// upload gave us nothing usable.
	resolvedType := strings.TrimSpace(contentType)
	if resolvedType == "" || resolvedType == "application/octet-stream" {
		resolvedType = mime.TypeByExtension(ext)
	}
	if resolvedType == "" {
		resolvedType = "application/octet-stream"
	}
	return coreservice.StoredArtifactResource{
		StorageKey:       storageKey,
		ResourceKind:     kind,
		MIMEType:         resolvedType,
		OriginalFilename: filepath.Base(filename),
		SizeBytes:        size,
	}, nil
}

func (s *FilesystemArtifactStore) ResourcePath(storageKey string) (string, error) {
	parts := strings.SplitN(storageKey, "/", 2)
	if len(parts) != 2 {
		return "", coreservice.ErrNotFound
	}
	baseDir := s.baseDir(parts[0])
	if baseDir == "" {
		return "", coreservice.ErrNotFound
	}
	path := filepath.Join(baseDir, filepath.Base(parts[1]))
	if _, err := os.Stat(path); err != nil {
		return "", coreservice.ErrNotFound
	}
	return path, nil
}

func (s *FilesystemArtifactStore) baseDir(kind string) string {
	switch kind {
	case "file":
		return s.uploadDir
	case "voice":
		return s.audioDir
	default:
		return ""
	}
}
