package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresCopilotStore persists copilot threads + the visible conversation (user + final
// assistant turns). Intra-turn tool traffic is streamed, not stored.
type PostgresCopilotStore struct{ pool *pgxpool.Pool }

func NewCopilotPostgres(pool *pgxpool.Pool) *PostgresCopilotStore {
	return &PostgresCopilotStore{pool: pool}
}

func (r *PostgresCopilotStore) CreateThread(ctx context.Context, id, userID, title string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO copilot_threads (id, user_id, title)
		VALUES ($1::uuid, $2::uuid, $3)
	`, id, userID, title)
	if err != nil {
		return fmt.Errorf("copilot create thread: %w", err)
	}
	return nil
}

func (r *PostgresCopilotStore) TouchThread(ctx context.Context, threadID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE copilot_threads SET updated_at = now() WHERE id = $1::uuid
	`, threadID)
	if err != nil {
		return fmt.Errorf("copilot touch thread: %w", err)
	}
	return nil
}

func (r *PostgresCopilotStore) ListThreads(ctx context.Context, userID string) ([]coreservice.CopilotThread, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, title, to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS updated_at,
		       pinned_at IS NOT NULL AS pinned
		  FROM copilot_threads
		 WHERE user_id = $1::uuid
		   AND archived_at IS NULL
		 ORDER BY pinned_at DESC NULLS LAST, updated_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("copilot list threads: %w", err)
	}
	defer rows.Close()
	out := make([]coreservice.CopilotThread, 0)
	for rows.Next() {
		var t coreservice.CopilotThread
		if err := rows.Scan(&t.ID, &t.Title, &t.UpdatedAt, &t.Pinned); err != nil {
			return nil, fmt.Errorf("copilot thread scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *PostgresCopilotStore) GetThread(ctx context.Context, userID, threadID string) (coreservice.CopilotThread, bool, error) {
	var t coreservice.CopilotThread
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, title, COALESCE(sdk_session_id, '') AS sdk_session_id,
		       to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS updated_at,
		       pinned_at IS NOT NULL AS pinned
		  FROM copilot_threads
		 WHERE id = $1::uuid AND user_id = $2::uuid
	`, threadID, userID).Scan(&t.ID, &t.Title, &t.SDKSessionID, &t.UpdatedAt, &t.Pinned)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return coreservice.CopilotThread{}, false, nil
		}
		return coreservice.CopilotThread{}, false, fmt.Errorf("copilot get thread: %w", err)
	}
	return t, true, nil
}

func (r *PostgresCopilotStore) RenameThread(ctx context.Context, userID, threadID, title string) (coreservice.CopilotThread, bool, error) {
	var t coreservice.CopilotThread
	err := r.pool.QueryRow(ctx, `
		UPDATE copilot_threads
		   SET title = $3,
		       updated_at = now()
		 WHERE id = $1::uuid
		   AND user_id = $2::uuid
		   AND archived_at IS NULL
		RETURNING id::text, title, COALESCE(sdk_session_id, '') AS sdk_session_id,
		          to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS updated_at,
		          pinned_at IS NOT NULL AS pinned
	`, threadID, userID, title).Scan(&t.ID, &t.Title, &t.SDKSessionID, &t.UpdatedAt, &t.Pinned)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return coreservice.CopilotThread{}, false, nil
		}
		return coreservice.CopilotThread{}, false, fmt.Errorf("copilot rename thread: %w", err)
	}
	return t, true, nil
}

func (r *PostgresCopilotStore) ArchiveThread(ctx context.Context, userID, threadID string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE copilot_threads
		   SET archived_at = now(),
		       updated_at = now()
		 WHERE id = $1::uuid
		   AND user_id = $2::uuid
		   AND archived_at IS NULL
	`, threadID, userID)
	if err != nil {
		return false, fmt.Errorf("copilot archive thread: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *PostgresCopilotStore) SetThreadPinned(ctx context.Context, userID, threadID string, pinned bool) (coreservice.CopilotThread, bool, error) {
	var t coreservice.CopilotThread
	err := r.pool.QueryRow(ctx, `
		UPDATE copilot_threads
		   SET pinned_at = CASE WHEN $3 THEN COALESCE(pinned_at, now()) ELSE NULL END
		 WHERE id = $1::uuid
		   AND user_id = $2::uuid
		   AND archived_at IS NULL
		RETURNING id::text, title, COALESCE(sdk_session_id, '') AS sdk_session_id,
		          to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS updated_at,
		          pinned_at IS NOT NULL AS pinned
	`, threadID, userID, pinned).Scan(&t.ID, &t.Title, &t.SDKSessionID, &t.UpdatedAt, &t.Pinned)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return coreservice.CopilotThread{}, false, nil
		}
		return coreservice.CopilotThread{}, false, fmt.Errorf("copilot set thread pinned: %w", err)
	}
	return t, true, nil
}

// SetThreadSDKSession stamps the Claude Agent SDK session id resumed on the next
// turn. Re-stamped after every turn (resume forks a new session id each time).
func (r *PostgresCopilotStore) SetThreadSDKSession(ctx context.Context, threadID, sessionID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE copilot_threads SET sdk_session_id = NULLIF($2, '') WHERE id = $1::uuid
	`, threadID, sessionID)
	if err != nil {
		return fmt.Errorf("copilot set sdk session: %w", err)
	}
	return nil
}

func (r *PostgresCopilotStore) AppendMessage(ctx context.Context, m coreservice.StoredCopilotMessage) error {
	citations := m.Citations
	if citations == nil {
		citations = []coreservice.Citation{}
	}
	payload, err := json.Marshal(citations)
	if err != nil {
		return fmt.Errorf("copilot marshal citations: %w", err)
	}
	meta := []byte(`{}`)
	if m.Meta != nil {
		if meta, err = json.Marshal(m.Meta); err != nil {
			return fmt.Errorf("copilot marshal meta: %w", err)
		}
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO copilot_messages (id, thread_id, role, content, citations, meta)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::jsonb, $6::jsonb)
	`, m.ID, m.ThreadID, m.Role, m.Content, string(payload), string(meta))
	if err != nil {
		return fmt.Errorf("copilot append message: %w", err)
	}
	return nil
}

func (r *PostgresCopilotStore) ListMessages(ctx context.Context, threadID string) ([]coreservice.CopilotMessage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, role, content, citations, meta,
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
		  FROM copilot_messages
		 WHERE thread_id = $1::uuid
		 ORDER BY seq ASC
	`, threadID)
	if err != nil {
		return nil, fmt.Errorf("copilot list messages: %w", err)
	}
	defer rows.Close()
	out := make([]coreservice.CopilotMessage, 0)
	for rows.Next() {
		var m coreservice.CopilotMessage
		var citations, meta []byte
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &citations, &meta, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("copilot message scan: %w", err)
		}
		if len(citations) > 0 {
			_ = json.Unmarshal(citations, &m.Citations)
		}
		if m.Citations == nil {
			m.Citations = []coreservice.Citation{}
		}
		// meta defaults to '{}' — only surface it when it actually carries something.
		if len(meta) > 2 {
			var parsed coreservice.CopilotMessageMeta
			if json.Unmarshal(meta, &parsed) == nil {
				m.Meta = &parsed
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
