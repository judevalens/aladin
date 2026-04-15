package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Dependencies struct {
	System    SystemService
	Sources   SourceService
	Artifacts ArtifactService
	Feed      FeedService
	Insights  InsightService
	Documents DocumentService
	Audio     AudioService
	Notes     NoteService
}

func NewDependencies(pool *pgxpool.Pool) Dependencies {
	sourceRepo := &pgSourceRepository{pool: pool}
	artifactRepo := &pgArtifactRepository{pool: pool}
	feedRepo := &pgFeedRepository{pool: pool}
	insightRepo := &pgInsightRepository{pool: pool}
	documentRepo := &pgDocumentRepository{pool: pool}
	noteRepo := &pgNoteRepository{pool: pool}
	systemRepo := &pgSystemRepository{pool: pool}

	return Dependencies{
		System:    &systemService{repo: systemRepo},
		Sources:   &sourceService{repo: sourceRepo},
		Artifacts: &artifactService{repo: artifactRepo},
		Feed:      &feedService{repo: feedRepo},
		Insights:  &insightService{repo: insightRepo},
		Documents: &documentService{repo: documentRepo, dir: uploadDir()},
		Audio:     &audioService{dir: audioDir()},
		Notes:     &noteService{repo: noteRepo},
	}
}

type SystemService interface {
	Ready(context.Context) error
	WorkerStatus(context.Context) (map[string]any, error)
}

type SourceService interface {
	List(context.Context) ([]sourceRecord, error)
	Create(context.Context, *sourcePayload) (sourceRecord, error)
	Delete(context.Context, string) error
}

type ArtifactService interface {
	List(context.Context) ([]artifactRecord, error)
	Children(context.Context, string, int, int) (map[string]any, error)
	Create(context.Context, string, string, string, string, string) error
	Delete(context.Context, string) error
}

type FeedService interface {
	List(context.Context, feedListParams) (map[string]any, error)
	Topics(context.Context) ([]string, error)
	Sources(context.Context) ([]feedSourceRecord, error)
	UpdateStatus(context.Context, string, string) error
}

type InsightService interface {
	List(context.Context, insightListParams) (map[string]any, error)
	Stats(context.Context) (map[string]any, error)
	UpdateStatus(context.Context, string, string) error
}

type DocumentService interface {
	Upload(context.Context, string, []byte) (documentRecord, error)
	List(context.Context) ([]documentRecord, error)
	FilePath(string) (string, error)
}

type AudioService interface {
	Upload(context.Context, string, io.Reader) (audioUploadRecord, error)
	FilePath(string) (string, error)
}

type NoteService interface {
	Create(context.Context, string, string) (noteRecord, error)
	Update(context.Context, string, *string, *string) (noteRecord, error)
	Delete(context.Context, string) error
	Related(context.Context, string, int) (map[string]any, error)
}

type SystemRepository interface {
	Ready(context.Context) error
	WorkerStatus(context.Context) (map[string]any, error)
}

type SourceRepository interface {
	EnsureDefaultUserAndGraph(context.Context) (string, error)
	List(context.Context) ([]sourceRecord, error)
	Create(context.Context, string, string, *sourcePayload) (sourceRecord, error)
	Delete(context.Context, string) error
}

type ArtifactRepository interface {
	List(context.Context) ([]artifactRecord, error)
	Children(context.Context, string, int, int) (map[string]any, error)
	Create(context.Context, string, string, string, string, string) error
	Delete(context.Context, string) error
}

type FeedRepository interface {
	List(context.Context, feedListParams) (map[string]any, error)
	Topics(context.Context) ([]string, error)
	Sources(context.Context) ([]feedSourceRecord, error)
	UpdateStatus(context.Context, string, string) error
}

type InsightRepository interface {
	List(context.Context, insightListParams) (map[string]any, error)
	Stats(context.Context) (map[string]any, error)
	UpdateStatus(context.Context, string, string) error
}

type DocumentRepository interface {
	Find(context.Context, string) (documentRecord, bool, error)
	Create(context.Context, documentRecord, int64) error
	List(context.Context) ([]documentRecord, error)
}

type NoteRepository interface {
	Create(context.Context, noteRecord) error
	Exists(context.Context, string) (bool, error)
	Update(context.Context, string, *string, *string) error
	Delete(context.Context, string) error
	Fetch(context.Context, string) (noteRecord, error)
	HasEmbedding(context.Context, string) (bool, error)
	Related(context.Context, string, int) ([]relatedRecord, error)
}

type sourceService struct{ repo SourceRepository }

func (s *sourceService) List(ctx context.Context) ([]sourceRecord, error) {
	if _, err := s.repo.EnsureDefaultUserAndGraph(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *sourceService) Create(ctx context.Context, payload *sourcePayload) (sourceRecord, error) {
	kgID, err := s.repo.EnsureDefaultUserAndGraph(ctx)
	if err != nil {
		return sourceRecord{}, err
	}
	return s.repo.Create(ctx, uuid.NewString(), kgID, payload)
}

func (s *sourceService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

type artifactService struct{ repo ArtifactRepository }

func (s *artifactService) List(ctx context.Context) ([]artifactRecord, error) {
	return s.repo.List(ctx)
}

func (s *artifactService) Children(ctx context.Context, id string, limit int, offset int) (map[string]any, error) {
	return s.repo.Children(ctx, id, limit, offset)
}

func (s *artifactService) Create(ctx context.Context, id string, kind string, label string, content string, sourceURL string) error {
	if strings.TrimSpace(id) == "" {
		id = "artifact-" + uuid.NewString()
	}
	return s.repo.Create(ctx, id, kind, label, content, sourceURL)
}

func (s *artifactService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

type feedService struct{ repo FeedRepository }

func (s *feedService) List(ctx context.Context, params feedListParams) (map[string]any, error) {
	return s.repo.List(ctx, params)
}

func (s *feedService) Topics(ctx context.Context) ([]string, error) {
	return s.repo.Topics(ctx)
}

func (s *feedService) Sources(ctx context.Context) ([]feedSourceRecord, error) {
	return s.repo.Sources(ctx)
}

func (s *feedService) UpdateStatus(ctx context.Context, id string, status string) error {
	return s.repo.UpdateStatus(ctx, id, status)
}

type insightService struct{ repo InsightRepository }

func (s *insightService) List(ctx context.Context, params insightListParams) (map[string]any, error) {
	return s.repo.List(ctx, params)
}

func (s *insightService) Stats(ctx context.Context) (map[string]any, error) {
	return s.repo.Stats(ctx)
}

func (s *insightService) UpdateStatus(ctx context.Context, id string, status string) error {
	return s.repo.UpdateStatus(ctx, id, status)
}

type documentService struct {
	repo DocumentRepository
	dir  string
}

func (s *documentService) Upload(ctx context.Context, filename string, content []byte) (documentRecord, error) {
	sum := sha256.Sum256(content)
	docID := "sha256:" + hex.EncodeToString(sum[:])

	existing, found, err := s.repo.Find(ctx, docID)
	if err != nil {
		return documentRecord{}, err
	}
	if found {
		existing.Status = "already_exists"
		return existing, nil
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return documentRecord{}, err
	}
	dest := filepath.Join(s.dir, docID+".pdf")
	if err := os.WriteFile(dest, content, 0o644); err != nil {
		return documentRecord{}, err
	}

	size := int64(len(content))
	now := time.Now().UTC()
	rec := documentRecord{
		ID:         docID,
		Filename:   filename,
		Title:      nil,
		Size:       &size,
		PageCount:  nil,
		UploadedAt: now.Format(time.RFC3339),
		Ingested:   true,
		Status:     "ingested",
	}
	if err := s.repo.Create(ctx, rec, now.Unix()); err != nil {
		return documentRecord{}, err
	}
	return rec, nil
}

func (s *documentService) List(ctx context.Context) ([]documentRecord, error) {
	return s.repo.List(ctx)
}

func (s *documentService) FilePath(id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", ErrNotFound
	}
	path := filepath.Join(s.dir, id+".pdf")
	if _, err := os.Stat(path); err != nil {
		return "", ErrNotFound
	}
	return path, nil
}

type audioService struct {
	dir string
}

type audioUploadRecord struct {
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
}

func (s *audioService) Upload(ctx context.Context, filename string, body io.Reader) (audioUploadRecord, error) {
	_ = ctx
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return audioUploadRecord{}, err
	}
	ext := strings.ToLower(filepath.Ext(filepath.Base(filename)))
	if ext == "" {
		ext = ".webm"
	}
	storedFilename := uuid.NewString() + ext
	dest := filepath.Join(s.dir, storedFilename)
	out, err := os.Create(dest)
	if err != nil {
		return audioUploadRecord{}, err
	}
	defer out.Close()
	if _, err := io.Copy(out, body); err != nil {
		return audioUploadRecord{}, err
	}
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return audioUploadRecord{
		Filename:    storedFilename,
		URL:         "/api/audio/" + storedFilename,
		ContentType: contentType,
	}, nil
}

func (s *audioService) FilePath(filename string) (string, error) {
	path := filepath.Join(s.dir, filepath.Base(filename))
	if _, err := os.Stat(path); err != nil {
		return "", ErrNotFound
	}
	return path, nil
}

type noteService struct{ repo NoteRepository }

func (s *noteService) Create(ctx context.Context, label string, content string) (noteRecord, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return noteRecord{}, badRequest("content is required")
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = content
		if len(label) > 60 {
			label = label[:60] + "..."
		}
		label = strings.ReplaceAll(label, "\n", " ")
	}
	now := time.Now().UTC()
	rec := noteRecord{
		ID:        "note-" + uuid.NewString(),
		Type:      "note",
		Label:     label,
		Content:   content,
		Metadata:  map[string]any{},
		Status:    "pending",
		CreatedAt: now.Format(time.RFC3339),
	}
	return rec, s.repo.Create(ctx, rec)
}

func (s *noteService) Update(ctx context.Context, id string, label *string, content *string) (noteRecord, error) {
	exists, err := s.repo.Exists(ctx, id)
	if err != nil {
		return noteRecord{}, err
	}
	if !exists {
		return noteRecord{}, ErrNotFound
	}
	if err := s.repo.Update(ctx, id, label, content); err != nil {
		return noteRecord{}, err
	}
	return s.repo.Fetch(ctx, id)
}

func (s *noteService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *noteService) Related(ctx context.Context, id string, limit int) (map[string]any, error) {
	hasEmbedding, err := s.repo.HasEmbedding(ctx, id)
	if err != nil || !hasEmbedding {
		return map[string]any{"results": []any{}, "message": "Note not yet embedded"}, nil
	}
	results, err := s.repo.Related(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	return map[string]any{"results": results}, nil
}

type systemService struct{ repo SystemRepository }

func (s *systemService) Ready(ctx context.Context) error {
	return s.repo.Ready(ctx)
}

func (s *systemService) WorkerStatus(ctx context.Context) (map[string]any, error) {
	return s.repo.WorkerStatus(ctx)
}

type pgSystemRepository struct{ pool *pgxpool.Pool }

func (r *pgSystemRepository) Ready(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *pgSystemRepository) WorkerStatus(ctx context.Context) (map[string]any, error) {
	var pendingPipeline int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM artifacts WHERE status IN ('pending', 'enriched', 'embedded')
	`).Scan(&pendingPipeline); err != nil {
		return nil, err
	}
	var activeCycles int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM sync_cycles WHERE status IN ('active', 'running')
	`).Scan(&activeCycles); err != nil {
		return nil, err
	}
	return map[string]any{
		"pipeline": map[string]any{
			"enriched":  0,
			"embedded":  0,
			"promoted":  0,
			"errors":    0,
			"last_tick": nil,
		},
		"queue": map[string]any{
			"pending":         activeCycles,
			"pendingPipeline": pendingPipeline,
			"deadJobs":        0,
		},
	}, nil
}

type pgSourceRepository struct{ pool *pgxpool.Pool }

func (r *pgSourceRepository) EnsureDefaultUserAndGraph(ctx context.Context) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, email, created_at)
		VALUES ($1::uuid, 'dev@aladin.local', now())
		ON CONFLICT (id) DO NOTHING
	`, defaultUserID)
	if err != nil {
		return "", err
	}

	var kgID string
	err = tx.QueryRow(ctx, `
		WITH existing AS (
		    SELECT id::text
		      FROM knowledge_graphs
		     WHERE user_id = $1::uuid
		     ORDER BY created_at ASC
		     LIMIT 1
		), inserted AS (
		    INSERT INTO knowledge_graphs (user_id, name, description)
		    SELECT $1::uuid, $2, 'Default workspace for persisted feed sources.'
		    WHERE NOT EXISTS (SELECT 1 FROM existing)
		    RETURNING id::text
		)
		SELECT id FROM existing
		UNION ALL
		SELECT id FROM inserted
		LIMIT 1
	`, defaultUserID, defaultKGName).Scan(&kgID)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return kgID, nil
}

func (r *pgSourceRepository) List(ctx context.Context) ([]sourceRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, name, type, sync_mode, sync_state, config,
		       auto_promote_threshold, suggest_threshold, created_at,
		       last_synced_at
		  FROM sources
		 WHERE user_id = $1::uuid
		 ORDER BY created_at DESC
	`, defaultUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []sourceRecord
	for rows.Next() {
		var rec sourceRecord
		var configJSON []byte
		var createdAt time.Time
		var lastSynced *time.Time
		if err := rows.Scan(
			&rec.ID, &rec.Name, &rec.Type, &rec.SyncMode, &rec.SyncState, &configJSON,
			&rec.AutoPromoteThreshold, &rec.SuggestThreshold, &createdAt, &lastSynced,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(configJSON, &rec.Config)
		rec.CreatedAt = createdAt.Format(time.RFC3339)
		if lastSynced != nil {
			v := lastSynced.Format(time.RFC3339)
			rec.LastSyncedAt = &v
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *pgSourceRepository) Create(ctx context.Context, sourceID string, kgID string, payload *sourcePayload) (sourceRecord, error) {
	configJSON, _ := json.Marshal(payload.Config)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sources (
		    id, user_id, kg_id, name, type, sync_mode, sync_state, config
		) VALUES (
		    $1::uuid, $2::uuid, $3::uuid, $4, $5, $6, 'active', $7::jsonb
		)
	`, sourceID, defaultUserID, kgID, payload.Name, payload.Type, payload.SyncMode, string(configJSON))
	if err != nil {
		return sourceRecord{}, err
	}

	row := r.pool.QueryRow(ctx, `
		SELECT id::text, name, type, sync_mode, sync_state, config,
		       auto_promote_threshold, suggest_threshold, created_at,
		       last_synced_at
		  FROM sources
		 WHERE id = $1::uuid
	`, sourceID)
	var rec sourceRecord
	var configOut []byte
	var createdAt time.Time
	var lastSynced *time.Time
	if err := row.Scan(
		&rec.ID, &rec.Name, &rec.Type, &rec.SyncMode, &rec.SyncState, &configOut,
		&rec.AutoPromoteThreshold, &rec.SuggestThreshold, &createdAt, &lastSynced,
	); err != nil {
		return sourceRecord{}, err
	}
	_ = json.Unmarshal(configOut, &rec.Config)
	rec.CreatedAt = createdAt.Format(time.RFC3339)
	if lastSynced != nil {
		v := lastSynced.Format(time.RFC3339)
		rec.LastSyncedAt = &v
	}
	return rec, nil
}

func (r *pgSourceRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM sources
		 WHERE id = $1::uuid AND user_id = $2::uuid
	`, id, defaultUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type pgArtifactRepository struct{ pool *pgxpool.Pool }

func (r *pgArtifactRepository) List(ctx context.Context) ([]artifactRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
		    a.id, a.type, a.label, a.content, a.source_url,
		    a.enrichment, a.metadata, a.created_at,
		    COUNT(c.id) AS child_count
		  FROM artifacts a
		  LEFT JOIN artifacts c ON c.parent_id = a.id AND c.status != 'superseded'
		 WHERE a.parent_id IS NULL
		   AND a.status != 'superseded'
		 GROUP BY a.id, a.type, a.label, a.content, a.source_url,
		          a.enrichment, a.metadata, a.created_at
		 ORDER BY a.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []artifactRecord
	for rows.Next() {
		rec, err := scanArtifactRow(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *pgArtifactRepository) Children(ctx context.Context, id string, limit int, offset int) (map[string]any, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, type, label, content, source_url,
		       enrichment, metadata, created_at
		  FROM artifacts
		 WHERE parent_id = $1
		   AND status != 'superseded'
		 ORDER BY COALESCE((metadata->>'score')::int, 0) DESC, created_at DESC
		 LIMIT $2 OFFSET $3
	`, id, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []artifactRecord
	for rows.Next() {
		rec, err := scanArtifactRow(rows, false)
		if err != nil {
			return nil, err
		}
		items = append(items, rec)
	}
	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM artifacts
		 WHERE parent_id = $1 AND status != 'superseded'
	`, id).Scan(&total); err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "total": total, "limit": limit, "offset": offset}, nil
}

func (r *pgArtifactRepository) Create(ctx context.Context, id string, kind string, label string, content string, sourceURL string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO artifacts (id, type, label, content, source_url, created_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (id) DO UPDATE SET
		    type = EXCLUDED.type,
		    label = EXCLUDED.label,
		    content = EXCLUDED.content,
		    source_url = EXCLUDED.source_url
	`, id, kind, label, content, nilIfEmpty(sourceURL))
	return err
}

func (r *pgArtifactRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM artifacts WHERE id = $1`, id)
	return err
}

type feedListParams struct {
	Limit      int
	Offset     int
	SourceType string
	Topic      string
	SavedOnly  bool
	Sort       string
}

type feedSourceRecord struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	SyncState    string  `json:"syncState"`
	LastSyncedAt *string `json:"lastSyncedAt"`
}

type pgFeedRepository struct{ pool *pgxpool.Pool }

func (r *pgFeedRepository) List(ctx context.Context, params feedListParams) (map[string]any, error) {
	query := `
		SELECT
		    a.id, a.type, a.label, LEFT(COALESCE(a.content, ''), 500) AS content,
		    a.source_url, a.enrichment, a.metadata, a.user_status, a.created_at,
		    s.type AS source_type, s.name AS source_name,
		    COALESCE(
		        LN(COALESCE((a.metadata->>'score')::float, 0) + 1) * 0.4
		        + GREATEST(0, 1 - EXTRACT(EPOCH FROM (now() - a.created_at)) / 2592000.0) * 0.6,
		        0
		    ) AS signal_score
		  FROM artifacts a
		  LEFT JOIN sources s ON s.id = a.source_id
		 WHERE a.status != 'superseded'
		   AND a.user_status IS DISTINCT FROM 'dismissed'
	`
	args := []any{}
	argPos := 1
	if params.SourceType != "" {
		query += ` AND s.type = $` + strconv.Itoa(argPos)
		args = append(args, params.SourceType)
		argPos++
	}
	if params.Topic != "" {
		query += ` AND EXISTS (
		    SELECT 1 FROM jsonb_array_elements_text(a.enrichment->'topics') t WHERE t = $` + strconv.Itoa(argPos) + `)`
		args = append(args, params.Topic)
		argPos++
	}
	if params.SavedOnly {
		query += ` AND a.user_status = 'saved'`
	}
	if params.Sort == "signal" {
		query += ` ORDER BY signal_score DESC, a.created_at DESC`
	} else {
		query += ` ORDER BY a.created_at DESC`
	}
	query += ` LIMIT $` + strconv.Itoa(argPos) + ` OFFSET $` + strconv.Itoa(argPos+1)
	args = append(args, params.Limit, params.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []feedItem
	for rows.Next() {
		var item feedItem
		var enrichment, metadata []byte
		var createdAt time.Time
		item.Metadata = map[string]any{}
		if err := rows.Scan(
			&item.ID, &item.Type, &item.Label, &item.Content, &item.SourceURL,
			&enrichment, &metadata, &item.UserStatus, &createdAt,
			&item.SourceType, &item.SourceName, &item.SignalScore,
		); err != nil {
			return nil, err
		}
		if len(enrichment) > 0 {
			_ = json.Unmarshal(enrichment, &item.Enrichment)
		}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &item.Metadata)
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}

	countQuery := `SELECT COUNT(*) FROM artifacts a LEFT JOIN sources s ON s.id = a.source_id WHERE a.status != 'superseded' AND a.user_status IS DISTINCT FROM 'dismissed'`
	countArgs := []any{}
	countPos := 1
	if params.SourceType != "" {
		countQuery += ` AND s.type = $` + strconv.Itoa(countPos)
		countArgs = append(countArgs, params.SourceType)
		countPos++
	}
	if params.Topic != "" {
		countQuery += ` AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(a.enrichment->'topics') t WHERE t = $` + strconv.Itoa(countPos) + `)`
		countArgs = append(countArgs, params.Topic)
		countPos++
	}
	if params.SavedOnly {
		countQuery += ` AND a.user_status = 'saved'`
	}
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "total": total, "limit": params.Limit, "offset": params.Offset}, nil
}

func (r *pgFeedRepository) Topics(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT jsonb_array_elements_text(enrichment->'topics') AS topic
		  FROM artifacts
		 WHERE enrichment IS NOT NULL
		   AND status != 'superseded'
		 ORDER BY topic
		 LIMIT 100
	`)
	if err != nil {
		return []string{}, nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var topic string
		if rows.Scan(&topic) == nil {
			out = append(out, topic)
		}
	}
	return out, nil
}

func (r *pgFeedRepository) Sources(ctx context.Context) ([]feedSourceRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, name, type, sync_state, last_synced_at
		  FROM sources
		 ORDER BY name
	`)
	if err != nil {
		return []feedSourceRecord{}, nil
	}
	defer rows.Close()
	var out []feedSourceRecord
	for rows.Next() {
		var rec feedSourceRecord
		var lastSynced *time.Time
		if rows.Scan(&rec.ID, &rec.Name, &rec.Type, &rec.SyncState, &lastSynced) == nil {
			if lastSynced != nil {
				v := lastSynced.Format(time.RFC3339)
				rec.LastSyncedAt = &v
			}
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *pgFeedRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	var err error
	if status == "" {
		_, err = r.pool.Exec(ctx, `UPDATE artifacts SET user_status = NULL WHERE id = $1`, id)
	} else {
		_, err = r.pool.Exec(ctx, `UPDATE artifacts SET user_status = $2 WHERE id = $1`, id, status)
	}
	return err
}

type insightListParams struct {
	Limit  int
	Offset int
	Type   string
	Status string
}

type pgInsightRepository struct{ pool *pgxpool.Pool }

func (r *pgInsightRepository) List(ctx context.Context, params insightListParams) (map[string]any, error) {
	query := `
		SELECT id::text, type, title, body, entity, topic,
		       artifact_ids, confidence, user_status, created_at
		  FROM insights
		 WHERE 1=1
	`
	args := []any{}
	argPos := 1
	if params.Type != "" {
		query += ` AND type = $` + strconv.Itoa(argPos)
		args = append(args, params.Type)
		argPos++
	}
	if params.Status != "" {
		query += ` AND user_status = $` + strconv.Itoa(argPos)
		args = append(args, params.Status)
		argPos++
	}
	query += ` ORDER BY confidence DESC, created_at DESC LIMIT $` + strconv.Itoa(argPos) + ` OFFSET $` + strconv.Itoa(argPos+1)
	args = append(args, params.Limit, params.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []insightRecord
	for rows.Next() {
		var item insightRecord
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Type, &item.Title, &item.Body, &item.Entity, &item.Topic, &item.ArtifactIDs, &item.Confidence, &item.UserStatus, &createdAt); err != nil {
			return nil, err
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}

	countQuery := `SELECT COUNT(*) FROM insights WHERE 1=1`
	countArgs := []any{}
	countPos := 1
	if params.Type != "" {
		countQuery += ` AND type = $` + strconv.Itoa(countPos)
		countArgs = append(countArgs, params.Type)
		countPos++
	}
	if params.Status != "" {
		countQuery += ` AND user_status = $` + strconv.Itoa(countPos)
		countArgs = append(countArgs, params.Status)
	}
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "total": total, "limit": params.Limit, "offset": params.Offset}, nil
}

func (r *pgInsightRepository) Stats(ctx context.Context) (map[string]any, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT type, user_status, COUNT(*) AS count
		  FROM insights
		 GROUP BY type, user_status
	`)
	if err != nil {
		return map[string]any{"byType": map[string]int{}, "byStatus": map[string]int{}}, nil
	}
	defer rows.Close()
	byType := map[string]int{}
	byStatus := map[string]int{}
	for rows.Next() {
		var t, status string
		var count int
		if rows.Scan(&t, &status, &count) == nil {
			byType[t] += count
			byStatus[status] += count
		}
	}
	return map[string]any{"byType": byType, "byStatus": byStatus}, nil
}

func (r *pgInsightRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE insights SET user_status = $2 WHERE id = $1::uuid`, id, status)
	return err
}

type pgDocumentRepository struct{ pool *pgxpool.Pool }

func (r *pgDocumentRepository) Find(ctx context.Context, id string) (documentRecord, bool, error) {
	var rec documentRecord
	var uploadedAt time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT id, filename, title, size, page_count, uploaded_at, ingested
		  FROM documents
		 WHERE id = $1
	`, id).Scan(&rec.ID, &rec.Filename, &rec.Title, &rec.Size, &rec.PageCount, &uploadedAt, &rec.Ingested)
	if errors.Is(err, pgx.ErrNoRows) {
		return documentRecord{}, false, nil
	}
	if err != nil {
		return documentRecord{}, false, err
	}
	rec.UploadedAt = uploadedAt.Format(time.RFC3339)
	return rec, true, nil
}

func (r *pgDocumentRepository) Create(ctx context.Context, rec documentRecord, uploadedAtUnix int64) error {
	return withUploadedAt(ctx, r.pool, rec, uploadedAtUnix)
}

func withUploadedAt(ctx context.Context, pool *pgxpool.Pool, rec documentRecord, uploadedAtUnix int64) error {
	uploadedAt := time.Unix(uploadedAtUnix, 0).UTC()
	var size any
	if rec.Size != nil {
		size = *rec.Size
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO documents (id, filename, title, size, page_count, uploaded_at, ingested, uploaded_by)
		VALUES ($1, $2, NULL, $3, NULL, $4, TRUE, $5::uuid)
	`, rec.ID, rec.Filename, size, uploadedAt, defaultUserID)
	return err
}

func (r *pgDocumentRepository) List(ctx context.Context) ([]documentRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, filename, title, size, page_count, uploaded_at, ingested
		  FROM documents
		 ORDER BY uploaded_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []documentRecord
	for rows.Next() {
		var rec documentRecord
		var uploadedAt time.Time
		if err := rows.Scan(&rec.ID, &rec.Filename, &rec.Title, &rec.Size, &rec.PageCount, &uploadedAt, &rec.Ingested); err != nil {
			return nil, err
		}
		rec.UploadedAt = uploadedAt.Format(time.RFC3339)
		out = append(out, rec)
	}
	return out, rows.Err()
}

type relatedRecord struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Label      string  `json:"label"`
	Content    string  `json:"content"`
	SourceURL  *string `json:"sourceUrl"`
	Enrichment any     `json:"enrichment"`
	Similarity float64 `json:"similarity"`
	CreatedAt  string  `json:"createdAt"`
}

type pgNoteRepository struct{ pool *pgxpool.Pool }

func (r *pgNoteRepository) Create(ctx context.Context, rec noteRecord) error {
	createdAt, err := time.Parse(time.RFC3339, rec.CreatedAt)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO artifacts (id, type, label, content, status, created_at)
		VALUES ($1, 'note', $2, $3, 'pending', $4)
	`, rec.ID, rec.Label, rec.Content, createdAt)
	return err
}

func (r *pgNoteRepository) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM artifacts WHERE id = $1 AND type = 'note')
	`, id).Scan(&exists)
	return exists, err
}

func (r *pgNoteRepository) Update(ctx context.Context, id string, label *string, content *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE artifacts
		   SET label = COALESCE(NULLIF(BTRIM($2), ''), label),
		       content = CASE
		           WHEN $3 IS NOT NULL AND BTRIM($3) <> content THEN BTRIM($3)
		           ELSE content
		       END,
		       status = CASE
		           WHEN $3 IS NOT NULL AND BTRIM($3) <> content THEN 'pending'
		           ELSE status
		       END,
		       enrichment = CASE
		           WHEN $3 IS NOT NULL AND BTRIM($3) <> content THEN NULL
		           ELSE enrichment
		       END,
		       embedding = CASE
		           WHEN $3 IS NOT NULL AND BTRIM($3) <> content THEN NULL
		           ELSE embedding
		       END
		 WHERE id = $1 AND type = 'note'
	`, id, derefString(label), derefString(content))
	return err
}

func (r *pgNoteRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM artifacts WHERE id = $1 AND type = 'note'`, id)
	return err
}

func (r *pgNoteRepository) Fetch(ctx context.Context, id string) (noteRecord, error) {
	var rec noteRecord
	var enrichment []byte
	var metadata []byte
	var createdAt time.Time
	rec.Metadata = map[string]any{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, type, label, content, enrichment, metadata, status, created_at
		  FROM artifacts
		 WHERE id = $1 AND type = 'note'
	`, id).Scan(&rec.ID, &rec.Type, &rec.Label, &rec.Content, &enrichment, &metadata, &rec.Status, &createdAt)
	if err != nil {
		return rec, err
	}
	if len(enrichment) > 0 {
		_ = json.Unmarshal(enrichment, &rec.Enrichment)
	}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &rec.Metadata)
	}
	rec.CreatedAt = createdAt.Format(time.RFC3339)
	return rec, nil
}

func (r *pgNoteRepository) HasEmbedding(ctx context.Context, id string) (bool, error) {
	var hasEmbedding bool
	err := r.pool.QueryRow(ctx, `
		SELECT embedding IS NOT NULL FROM artifacts WHERE id = $1
	`, id).Scan(&hasEmbedding)
	return hasEmbedding, err
}

func (r *pgNoteRepository) Related(ctx context.Context, id string, limit int) ([]relatedRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
		    a.id, a.type, a.label, LEFT(COALESCE(a.content, ''), 300) AS content,
		    a.source_url, a.enrichment, a.created_at,
		    1 - (a.embedding <=> (SELECT embedding FROM artifacts WHERE id = $1)) AS similarity
		  FROM artifacts a
		 WHERE a.embedding IS NOT NULL
		   AND a.status != 'superseded'
		   AND a.id != $1
		 ORDER BY a.embedding <=> (SELECT embedding FROM artifacts WHERE id = $1)
		 LIMIT $2
	`, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []relatedRecord
	for rows.Next() {
		var item relatedRecord
		var enrichment []byte
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Type, &item.Label, &item.Content, &item.SourceURL, &enrichment, &createdAt, &item.Similarity); err != nil {
			return nil, err
		}
		if len(enrichment) > 0 {
			_ = json.Unmarshal(enrichment, &item.Enrichment)
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		results = append(results, item)
	}
	return results, rows.Err()
}
