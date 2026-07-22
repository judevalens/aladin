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
		SELECT id::text, title, to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS updated_at
		  FROM copilot_threads
		 WHERE user_id = $1::uuid
		 ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("copilot list threads: %w", err)
	}
	defer rows.Close()
	out := make([]coreservice.CopilotThread, 0)
	for rows.Next() {
		var t coreservice.CopilotThread
		if err := rows.Scan(&t.ID, &t.Title, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("copilot thread scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *PostgresCopilotStore) GetThread(ctx context.Context, userID, threadID string) (coreservice.CopilotThread, bool, error) {
	var t coreservice.CopilotThread
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, title, to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS updated_at
		  FROM copilot_threads
		 WHERE id = $1::uuid AND user_id = $2::uuid
	`, threadID, userID).Scan(&t.ID, &t.Title, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return coreservice.CopilotThread{}, false, nil
		}
		return coreservice.CopilotThread{}, false, fmt.Errorf("copilot get thread: %w", err)
	}
	return t, true, nil
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
	_, err = r.pool.Exec(ctx, `
		INSERT INTO copilot_messages (id, thread_id, role, content, citations)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::jsonb)
	`, m.ID, m.ThreadID, m.Role, m.Content, string(payload))
	if err != nil {
		return fmt.Errorf("copilot append message: %w", err)
	}
	return nil
}

func (r *PostgresCopilotStore) ListMessages(ctx context.Context, threadID string) ([]coreservice.CopilotMessage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, role, content, citations,
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
		var citations []byte
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &citations, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("copilot message scan: %w", err)
		}
		if len(citations) > 0 {
			_ = json.Unmarshal(citations, &m.Citations)
		}
		if m.Citations == nil {
			m.Citations = []coreservice.Citation{}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
