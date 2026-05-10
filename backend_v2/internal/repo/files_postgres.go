package repo

import (
	"context"
	"errors"
	"time"

	artifactservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresArtifactRepository) CreateFile(ctx context.Context, rec artifactservice.FileRecord) error {
	userID, err := r.userID(ctx)
	if err != nil {
		return err
	}
	uploadedAt, err := time.Parse(time.RFC3339, rec.UploadedAt)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO files (id, user_id, storage_key, uploaded_at)
		VALUES ($1, $2::uuid, $3, $4)
	`, rec.ID, userID, rec.StorageKey, uploadedAt)
	return err
}

func (r *PostgresArtifactRepository) GetFile(ctx context.Context, id string) (artifactservice.FileRecord, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return artifactservice.FileRecord{}, err
	}
	var rec artifactservice.FileRecord
	var uploadedAt time.Time
	err = r.pool.QueryRow(ctx, `
		SELECT id, storage_key, uploaded_at
		  FROM files
		 WHERE id = $1 AND user_id = $2::uuid
	`, id, userID).Scan(&rec.ID, &rec.StorageKey, &uploadedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return artifactservice.FileRecord{}, artifactservice.ErrNotFound
	}
	if err != nil {
		return artifactservice.FileRecord{}, err
	}
	rec.UploadedAt = uploadedAt.UTC().Format(time.RFC3339)
	rec.URL = "/api/files/" + rec.ID + "/resource"
	return rec, nil
}
