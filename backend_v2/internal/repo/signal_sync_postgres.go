package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// signal_sync_postgres.go — claims as a SYNCED entity ("signal" kind). New/updated claims push
// to the owner's local store as data_event frames (the same durable outbox lane the workspace
// tree rides), so the Signals surface reads from local SQLite and updates on push. The light
// projection is the ClaimSignal card shape: text, polarity, subjects, evidence tally, edge counts.

const signalEntityKind = "signal"

// lightSignalData is the JSON `data` payload of a signal upsert frame — exactly what a Signals
// card renders. Mirrors service.ClaimSignal (kept independent: this is the wire shape).
type lightSignalData struct {
	ID          string             `json:"id"`
	Text        string             `json:"text"`
	Polarity    string             `json:"polarity"`
	TrustTier   string             `json:"trustTier"`
	Subjects    []signalSubjectRef `json:"subjects"`
	AssertCount int                `json:"assertCount"`
	DenyCount   int                `json:"denyCount"`
	Supports    int                `json:"supports"`
	Contradicts int                `json:"contradicts"`
	Qualifies   int                `json:"qualifies"`
	SignalScore float64            `json:"signalScore"`
}

type signalSubjectRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// lightSignalSelect projects a claim to its card + the staleness seq. Callers append a row filter
// (` AND c.id = $1`) or an ordering. Mirrors the Signals List CTE for a single/all rows.
const lightSignalSelect = `
	SELECT c.id::text, c.canonical_text, c.polarity, c.trust_tier, c.seq,
	       COALESCE(mc.assert_count, 0), COALESCE(mc.deny_count, 0),
	       COALESCE(ec.supports, 0), COALESCE(ec.contradicts, 0), COALESCE(ec.qualifies, 0),
	       COALESCE((
			SELECT json_agg(json_build_object('id', en.id::text, 'name', en.canonical_name) ORDER BY en.canonical_name)
			  FROM claim_subjects cs JOIN entities en ON en.id = cs.entity_id
			 WHERE cs.claim_id = c.id
	       ), '[]'::json)
	  FROM claims c
	  LEFT JOIN (
		SELECT claim_id,
		       count(*) FILTER (WHERE stance = 'assert') AS assert_count,
		       count(*) FILTER (WHERE stance = 'deny')   AS deny_count
		  FROM claim_mentions GROUP BY claim_id
	  ) mc ON mc.claim_id = c.id
	  LEFT JOIN (
		SELECT cid AS claim_id,
		       count(*) FILTER (WHERE type = 'supports')    AS supports,
		       count(*) FILTER (WHERE type = 'contradicts') AS contradicts,
		       count(*) FILTER (WHERE type = 'qualifies')   AS qualifies
		  FROM (
			SELECT from_claim_id AS cid, type FROM claim_edges WHERE status <> 'rejected'
			UNION ALL
			SELECT to_claim_id   AS cid, type FROM claim_edges WHERE status <> 'rejected'
		  ) e GROUP BY cid
	  ) ec ON ec.claim_id = c.id
	 WHERE c.scope = 'shared'`

// scanLightSignal maps one projected claim row to an upsert FrameEntity.
func scanLightSignal(row scanner) (service.FrameEntity, error) {
	var (
		d        lightSignalData
		seq      int64
		subjects []byte
	)
	if err := row.Scan(
		&d.ID, &d.Text, &d.Polarity, &d.TrustTier, &seq,
		&d.AssertCount, &d.DenyCount,
		&d.Supports, &d.Contradicts, &d.Qualifies,
		&subjects,
	); err != nil {
		return service.FrameEntity{}, err
	}
	if len(subjects) > 0 {
		if err := json.Unmarshal(subjects, &d.Subjects); err != nil {
			return service.FrameEntity{}, err
		}
	}
	d.SignalScore = float64(d.AssertCount+d.DenyCount) +
		2*float64(min(d.AssertCount, d.DenyCount)) +
		float64(d.Supports+d.Contradicts+d.Qualifies)
	data, err := json.Marshal(d)
	if err != nil {
		return service.FrameEntity{}, err
	}
	return service.FrameEntity{
		EntityKind: signalEntityKind,
		EntityID:   d.ID,
		Seq:        uint64(seq),
		Op:         service.OpUpsert,
		Data:       data,
	}, nil
}

func lightSignalByID(ctx context.Context, q rowQuerier, claimID string) (service.FrameEntity, error) {
	ent, err := scanLightSignal(q.QueryRow(ctx, lightSignalSelect+` AND c.id = $1::uuid`, claimID))
	if err != nil {
		return service.FrameEntity{}, fmt.Errorf("sync: light signal %s: %w", claimID, err)
	}
	return ent, nil
}

// SignalSyncRepo emits claim frames into the durable outbox (the data_event lane).
type SignalSyncRepo struct{ pool *pgxpool.Pool }

func NewSignalSyncRepo(pool *pgxpool.Pool) *SignalSyncRepo {
	return &SignalSyncRepo{pool: pool}
}

// EmitForRecord pushes one frame carrying every claim this record touched (asserted/denied) to the
// users who should see it — the **subscribers of the record's source** (a public sync record is
// ownerless; it was ingested because a user subscribed) PLUS the record owner if set (authored
// records). New claims AND existing ones whose tally/score just changed are included. One tx: bump
// each claim's seq once, read its projection, append the frame to each target user's outbox.
// No targets ⇒ no-op. Idempotent enough to re-run on task retry.
func (r *SignalSyncRepo) EmitForRecord(ctx context.Context, recordID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	targets, err := recordSignalTargets(ctx, tx, recordID)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil // no subscriber/owner to sync to
	}

	ids, err := recordClaimIDs(ctx, tx, recordID)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}

	entities := make([]service.FrameEntity, 0, len(ids))
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `UPDATE claims SET seq = seq + 1 WHERE id = $1::uuid`, id); err != nil {
			return fmt.Errorf("sync: bump claim seq %s: %w", id, err)
		}
		ent, err := lightSignalByID(ctx, tx, id)
		if err != nil {
			return err
		}
		entities = append(entities, ent)
	}
	frame := service.Frame{Entities: entities}
	for _, u := range targets { // targets are sorted → deterministic lock order
		if err := LockUser(ctx, tx, u); err != nil {
			return err
		}
		if err := appendOutboxEvent(ctx, tx, u, frame); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// BackfillAll re-emits signal frames for every record that has produced claims, to that record's
// subscribers — replaying the live push for the EXISTING claim set, so an already-synced client
// receives claims created before the sync was wired (or before the targeting fix). Idempotent: each
// EmitForRecord bumps seq, and the client's seq guard applies the latest. Returns records processed.
func (r *SignalSyncRepo) BackfillAll(ctx context.Context) (int, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT source_id FROM claim_mentions WHERE source_kind = 'record'`)
	if err != nil {
		return 0, fmt.Errorf("sync: backfill record ids: %w", err)
	}
	var recordIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		recordIDs = append(recordIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for i, id := range recordIDs {
		if err := r.EmitForRecord(ctx, id); err != nil {
			return i, fmt.Errorf("sync: backfill emit record %s: %w", id, err)
		}
	}
	return len(recordIDs), nil
}

// recordSignalTargets resolves the users who should receive a record's claim frames: the
// subscribers of the record's provider stream, plus the record owner if set. Sorted + deduped.
func recordSignalTargets(ctx context.Context, tx pgx.Tx, recordID string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT ss.user_id::text
		  FROM source_subscriptions ss
		  JOIN records r ON r.provider_stream_id = ss.provider_stream_id
		 WHERE r.id = $1 AND ss.user_id IS NOT NULL
		UNION
		SELECT owner_user_id::text FROM records WHERE id = $1 AND owner_user_id IS NOT NULL
		ORDER BY 1
	`, recordID)
	if err != nil {
		return nil, fmt.Errorf("sync: record signal targets: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func recordClaimIDs(ctx context.Context, tx pgx.Tx, recordID string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT claim_id::text FROM claim_mentions
		 WHERE source_kind = 'record' AND source_id = $1
	`, recordID)
	if err != nil {
		return nil, fmt.Errorf("sync: record claim ids: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SignalSyncSource is the cold-start snapshot provider for claims. Claims are shared, so every
// user's snapshot is the full shared claim set. Implements service.SyncSource.
type SignalSyncSource struct{ pool *pgxpool.Pool }

func NewSignalSyncSource(pool *pgxpool.Pool) *SignalSyncSource {
	return &SignalSyncSource{pool: pool}
}

func (s *SignalSyncSource) EntityKind() string { return signalEntityKind }

func (s *SignalSyncSource) Snapshot(ctx context.Context, _ string) ([]service.FrameEntity, error) {
	rows, err := s.pool.Query(ctx, lightSignalSelect+` ORDER BY c.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("sync: signal snapshot query: %w", err)
	}
	defer rows.Close()

	out := make([]service.FrameEntity, 0)
	for rows.Next() {
		ent, err := scanLightSignal(rows)
		if err != nil {
			return nil, fmt.Errorf("sync: signal snapshot scan: %w", err)
		}
		out = append(out, ent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sync: signal snapshot rows: %w", err)
	}
	return out, nil
}
