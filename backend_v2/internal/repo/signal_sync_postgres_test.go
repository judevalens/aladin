package repo

import (
	"context"
	"encoding/json"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"
	"aladin/backend_v2/internal/service"

	"github.com/google/uuid"
)

func TestSignalSyncRepo_EmitForRecord(t *testing.T) {
	t.Parallel()
	dsn := dbtest.RequireTestDSN(t)

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	recordID := "rec-" + tag
	entID := uuid.NewString()
	var claimID string

	if _, err := pool.Exec(ctx, `INSERT INTO users(id,email,created_at) VALUES($1,$2,now())`, userID, "sync-"+userID+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO entities (id, scope, kind, canonical_name, normalized_key) VALUES ($1,'shared','org',$2,$3)`,
		entID, "Acme "+tag, "acme-"+tag); err != nil {
		t.Fatalf("seed entity: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO claims (scope, canonical_text, polarity) VALUES ('shared',$1,'assert') RETURNING id::text`,
		"Acme ships "+tag).Scan(&claimID); err != nil {
		t.Fatalf("seed claim: %v", err)
	}
	// The trigger record, owned by userID — so target resolution (subscribers ∪ owner) yields userID.
	if _, err := pool.Exec(ctx, `INSERT INTO records (id, type, label, content, owner_user_id) VALUES ($1,'link','x','x',$2)`,
		recordID, userID); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM records WHERE id = $1`, recordID)
		_, _ = pool.Exec(bg, `DELETE FROM claims WHERE id = $1::uuid`, claimID)
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id = $1`, entID)
		_, _ = pool.Exec(bg, `DELETE FROM outbox_events WHERE user_id = $1`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1`, userID)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO claim_subjects (claim_id, entity_id) VALUES ($1::uuid,$2::uuid)`, claimID, entID); err != nil {
		t.Fatalf("seed subject: %v", err)
	}
	// Three mentions from three different records (the unique key is source+claim): the trigger
	// record asserts, two others assert/deny — so the claim's aggregate tally is 2 assert / 1 deny.
	mentions := []struct{ src, stance string }{
		{recordID, "assert"}, {"rec2-" + tag, "assert"}, {"rec3-" + tag, "deny"},
	}
	for _, m := range mentions {
		if _, err := pool.Exec(ctx, `INSERT INTO claim_mentions (claim_id, source_kind, source_id, stance) VALUES ($1::uuid,'record',$2,$3)`,
			claimID, m.src, m.stance); err != nil {
			t.Fatalf("seed mention: %v", err)
		}
	}

	if err := NewSignalSyncRepo(pool).EmitForRecord(ctx, recordID); err != nil {
		t.Fatalf("EmitForRecord: %v", err)
	}

	// The outbox holds one data_event frame with the claim as a signal upsert.
	var payload []byte
	if err := pool.QueryRow(ctx, `
		SELECT payload FROM outbox_events WHERE user_id=$1::uuid AND type='data_event' ORDER BY xid DESC LIMIT 1
	`, userID).Scan(&payload); err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	var frame service.Frame
	if err := json.Unmarshal(payload, &frame); err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	if len(frame.Entities) != 1 {
		t.Fatalf("frame entities = %d, want 1", len(frame.Entities))
	}
	ent := frame.Entities[0]
	if ent.EntityKind != "signal" || ent.EntityID != claimID || ent.Op != service.OpUpsert {
		t.Fatalf("entity = %+v, want signal/upsert/%s", ent, claimID)
	}
	if ent.Seq != 1 {
		t.Fatalf("seq = %d, want 1 (bumped from 0)", ent.Seq)
	}
	var d lightSignalData
	if err := json.Unmarshal(ent.Data, &d); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if d.AssertCount != 2 || d.DenyCount != 1 {
		t.Fatalf("assert/deny = %d/%d, want 2/1", d.AssertCount, d.DenyCount)
	}
	if len(d.Subjects) != 1 || d.Subjects[0].Name != "Acme "+tag {
		t.Fatalf("subjects = %+v", d.Subjects)
	}
	// score = (2+1) + 2*min(2,1) + 0 = 5
	if d.SignalScore != 5 {
		t.Fatalf("signalScore = %v, want 5", d.SignalScore)
	}

	// Cold-start snapshot includes the claim as a signal frame entity.
	ents, err := NewSignalSyncSource(pool).Snapshot(ctx, userID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	found := false
	for _, e := range ents {
		if e.EntityID == claimID && e.EntityKind == "signal" {
			found = true
		}
	}
	if !found {
		t.Fatalf("snapshot missing the seeded claim %s", claimID)
	}
}
