package postgres

import (
	"context"
	"errors"
	"time"

	"aladin/backend_v2/internal/file"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) CreateFile(ctx context.Context, rec file.FileRecord) error {
	userID, err := userID(ctx)
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

func (r *Repository) GetFile(ctx context.Context, id string) (file.FileRecord, error) {
	userID, err := userID(ctx)
	if err != nil {
		return file.FileRecord{}, err
	}
	var rec file.FileRecord
	var uploadedAt time.Time
	err = r.pool.QueryRow(ctx, `
		SELECT id, storage_key, uploaded_at
		  FROM files
		 WHERE id = $1 AND user_id = $2::uuid
	`, id, userID).Scan(&rec.ID, &rec.StorageKey, &uploadedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return file.FileRecord{}, coreservice.ErrNotFound
	}
	if err != nil {
		return file.FileRecord{}, err
	}
	rec.UploadedAt = uploadedAt.UTC().Format(time.RFC3339)
	rec.URL = "/api/files/" + rec.ID + "/resource"
	return rec, nil
}

func userID(ctx context.Context) (string, error) {
	principal, err := coreservice.RequirePrincipal(ctx)
	if err != nil {
		return "", err
	}
	return principal.UserID, nil
}
