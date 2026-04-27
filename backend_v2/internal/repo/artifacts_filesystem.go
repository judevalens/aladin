package repo

import (
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

func (s *FilesystemArtifactStore) SaveDocument(id string, content []byte) error {
	if err := os.MkdirAll(s.uploadDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.uploadDir, id+".pdf"), content, 0o644)
}

func (s *FilesystemArtifactStore) DocumentPath(id string) (string, error) {
	path := filepath.Join(s.uploadDir, id+".pdf")
	if _, err := os.Stat(path); err != nil {
		return "", coreservice.ErrNotFound
	}
	return path, nil
}

func (s *FilesystemArtifactStore) SaveAudio(filename string, body io.Reader) (string, string, error) {
	if err := os.MkdirAll(s.audioDir, 0o755); err != nil {
		return "", "", err
	}
	ext := strings.ToLower(filepath.Ext(filepath.Base(filename)))
	if ext == "" {
		ext = ".webm"
	}
	storedFilename := coreservice.NewID("audio-", ext)
	dest := filepath.Join(s.audioDir, storedFilename)
	out, err := os.Create(dest)
	if err != nil {
		return "", "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, body); err != nil {
		return "", "", err
	}
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return storedFilename, contentType, nil
}

func (s *FilesystemArtifactStore) AudioPath(filename string) (string, error) {
	path := filepath.Join(s.audioDir, filepath.Base(filename))
	if _, err := os.Stat(path); err != nil {
		return "", coreservice.ErrNotFound
	}
	return path, nil
}
