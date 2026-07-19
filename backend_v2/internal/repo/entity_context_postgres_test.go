package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/entities"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/google/uuid"
)

// TestEntityContext_ReadPath covers the Entity Context surface's read path (Phase B):
// identity + typed edges (both directions, oriented from the focused entity) + context
// derived VERBATIM from real material (a record that mentions it → quote; a page block
// that @mentions it → your note).
func TestEntityContext_ReadPath(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	artID := "ec-art-" + uuid.NewString()
	recID := "ec-rec-" + uuid.NewString()
	focusName := "Moat" + tag
	targetName := "Lockin" + tag
	inboundName := "Aggregation" + tag

	quoteBody := "memory is the moat. the model is a commodity now."
	noteBody := "This is the whole thesis, turned inward — @" + focusName + " is the bet."

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "u-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO artifacts (id, user_id, type, title, content)
		VALUES ($1, $2::uuid, 'page', 'Thesis page', '')
	`, artID, userID); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	// ReplaceMentions now emits a node frame → needs a principal + the artifact's tree_nodes row.
	ctx = adminContext(userID)
	if _, err := pool.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, kind, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, 'artifact', $1, 0, now(), now())
	`, artID, userID); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO records (id, type, label, content, status, source_revision, provider, metadata)
		VALUES ($1, 'story', 'post', $2, 'enriched', 1, 'bluesky', '{"author_handle":"swyx"}'::jsonb)
	`, recID, quoteBody); err != nil {
		t.Fatalf("seed record: %v", err)
	}

	// Three entities: the focused one, an outbound target, an inbound source.
	var focusID, targetID, inboundID string
	for _, e := range []struct {
		name string
		into *string
	}{{focusName, &focusID}, {targetName, &targetID}, {inboundName, &inboundID}} {
		if err := pool.QueryRow(ctx, `
			INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier, gist)
			VALUES ('shared', 'concept', $1, $2, 'believed', 'A one-line gist.')
			RETURNING id::text
		`, e.name, entities.Normalize(e.name)).Scan(e.into); err != nil {
			t.Fatalf("seed entity %s: %v", e.name, err)
		}
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM relationships WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM records WHERE id = $1`, recID)
		_, _ = pool.Exec(bg, `DELETE FROM artifacts WHERE id = $1`, artID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id IN ($1::uuid, $2::uuid, $3::uuid)`,
			focusID, targetID, inboundID)
	})

	// The record mentions the focused entity → a quote.
	if _, err := pool.Exec(ctx, `
		INSERT INTO entity_mentions (record_id, entity_id, surface, resolver)
		VALUES ($1, $2::uuid, $3, 'alias')
	`, recID, focusID, focusName); err != nil {
		t.Fatalf("seed entity mention: %v", err)
	}

	tagRepo := NewEntityTagPostgres(pool)
	ecRepo := NewEntityContextPostgres(pool)

	// The page @mentions it, carrying the block's text → your note.
	if err := tagRepo.ReplaceMentions(ctx, artID, []coreservice.MentionRef{
		{EntityID: focusID, BlockID: "b1", Surface: focusName, Snippet: noteBody},
	}); err != nil {
		t.Fatalf("replace mentions: %v", err)
	}

	// Edges: focus --enables--> target (outbound), inbound --part_of--> focus.
	if err := ecRepo.InsertEdge(ctx, coreservice.DrawEdgeInput{
		OwnerUserID: userID, FromID: focusID, ToID: targetID,
		Rel: coreservice.RelEnables, Why: "the lock-in IS the moat",
	}); err != nil {
		t.Fatalf("insert outbound edge: %v", err)
	}
	if err := ecRepo.InsertEdge(ctx, coreservice.DrawEdgeInput{
		OwnerUserID: userID, FromID: inboundID, ToID: focusID,
		Rel: coreservice.RelPartOf, Why: "a special case",
	}); err != nil {
		t.Fatalf("insert inbound edge: %v", err)
	}

	svc := coreservice.NewEntityContextService(ecRepo, db.NewEntityRepository(pool))
	out, err := svc.Get(ctx, userID, focusID)
	if err != nil {
		t.Fatalf("get context: %v", err)
	}

	// Identity + the composed provenance line.
	if out.Entity.Name != focusName || out.Entity.Kind != "concept" || out.Entity.Gist == "" {
		t.Fatalf("identity = %+v", out.Entity)
	}
	if out.Entity.Since != "tracked today · 2 pieces of context" {
		t.Fatalf("provenance line = %q", out.Entity.Since)
	}

	// Both edges, each oriented FROM the focused entity: the inbound `part_of` is read
	// from this end as `instance`.
	if len(out.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %+v", out.Edges)
	}
	byTarget := map[string]coreservice.EntityEdge{}
	for _, e := range out.Edges {
		byTarget[e.To] = e
	}
	if e := byTarget[targetName]; e.Rel != coreservice.RelEnables || e.ToID != targetID ||
		e.Why != "the lock-in IS the moat" || e.Origin != "you" {
		t.Fatalf("outbound edge = %+v", e)
	}
	if e := byTarget[inboundName]; e.Rel != coreservice.RelInstance {
		t.Fatalf("inbound part_of should read as instance from this end, got %+v", e)
	}

	// Context: the quote and the note, both verbatim, newest first.
	if len(out.Context) != 2 {
		t.Fatalf("expected 2 context items, got %+v", out.Context)
	}
	var quote, note *coreservice.EntityContextItem
	for i := range out.Context {
		switch out.Context[i].Type {
		case "quote":
			quote = &out.Context[i]
		case "note":
			note = &out.Context[i]
		}
	}
	if quote == nil || quote.Body != quoteBody {
		t.Fatalf("quote body must be verbatim, got %+v", quote)
	}
	if quote.Who != "@swyx" || quote.Platform != "bluesky" {
		t.Fatalf("quote attribution = %+v", quote)
	}
	if note == nil || note.Body != noteBody {
		t.Fatalf("note body must be the block's text verbatim, got %+v", note)
	}
	if note.Who != "me" || note.Platform != "" || note.SourceID != artID {
		t.Fatalf("note attribution = %+v", note)
	}
}

// TestEntityContext_MergedIdResolvesToRoot: a link to an entity that was merged away
// lands on its canonical root — the payoff of the immortal-ids contract.
func TestEntityContext_MergedIdResolvesToRoot(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	rootName := "Root" + tag
	mergedName := "Merged" + tag

	var rootID, mergedID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier)
		VALUES ('shared', 'org', $1, $2, 'believed') RETURNING id::text
	`, rootName, entities.Normalize(rootName)).Scan(&rootID); err != nil {
		t.Fatalf("seed root: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO entities (scope, kind, canonical_name, normalized_key, canonical_root_id, trust_tier)
		VALUES ('shared', 'org', $1, $2, $3::uuid, 'placeholder') RETURNING id::text
	`, mergedName, entities.Normalize(mergedName), rootID).Scan(&mergedID); err != nil {
		t.Fatalf("seed merged: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id IN ($1::uuid, $2::uuid)`, mergedID, rootID)
	})

	svc := coreservice.NewEntityContextService(NewEntityContextPostgres(pool), db.NewEntityRepository(pool))
	out, err := svc.Get(ctx, uuid.NewString(), mergedID)
	if err != nil {
		t.Fatalf("get by merged id: %v", err)
	}
	if out.Entity.ID != rootID || out.Entity.Name != rootName {
		t.Fatalf("expected merged id to resolve to root %s/%s, got %+v", rootID, rootName, out.Entity)
	}
	if out.Entity.Placeholder {
		t.Fatal("resolved root is a real entity, must not be flagged placeholder")
	}
}

// TestEntityContext_DrawEdgeGuards covers the write path's integrity boundary: since
// `relationships` deliberately has no FKs on its polymorphic endpoints, the service is
// what stops junk edges. Also: redrawing the same edge updates its why (idempotent).
func TestEntityContext_DrawEdgeGuards(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	aName, bName := "EdgeA"+tag, "EdgeB"+tag

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "u-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var aID, bID string
	for _, e := range []struct {
		name string
		into *string
	}{{aName, &aID}, {bName, &bID}} {
		if err := pool.QueryRow(ctx, `
			INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier)
			VALUES ('shared', 'concept', $1, $2, 'believed') RETURNING id::text
		`, e.name, entities.Normalize(e.name)).Scan(e.into); err != nil {
			t.Fatalf("seed entity: %v", err)
		}
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM relationships WHERE user_id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id IN ($1::uuid, $2::uuid)`, aID, bID)
	})

	svc := coreservice.NewEntityContextService(NewEntityContextPostgres(pool), db.NewEntityRepository(pool))

	// Rejected: unknown relation type (a rel outside the surface's vocabulary).
	if err := svc.DrawEdge(ctx, coreservice.DrawEdgeInput{
		OwnerUserID: userID, FromID: aID, ToID: bID, Rel: "vibes_with",
	}); err == nil {
		t.Fatal("expected unknown rel type rejected")
	}
	// Rejected: self-edge.
	if err := svc.DrawEdge(ctx, coreservice.DrawEdgeInput{
		OwnerUserID: userID, FromID: aID, ToID: aID, Rel: coreservice.RelEnables,
	}); err == nil {
		t.Fatal("expected self-edge rejected")
	}
	// Rejected: target that doesn't exist (the missing-FK guard).
	if err := svc.DrawEdge(ctx, coreservice.DrawEdgeInput{
		OwnerUserID: userID, FromID: aID, ToID: uuid.NewString(), Rel: coreservice.RelEnables,
	}); err == nil {
		t.Fatal("expected nonexistent target rejected")
	}

	// Accepted, then redrawn with a new why → one edge, updated.
	for _, why := range []string{"first reason", "sharper reason"} {
		if err := svc.DrawEdge(ctx, coreservice.DrawEdgeInput{
			OwnerUserID: userID, FromID: aID, ToID: bID, Rel: coreservice.RelEnables, Why: why,
		}); err != nil {
			t.Fatalf("draw edge (%s): %v", why, err)
		}
	}
	out, err := svc.Get(ctx, userID, aID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(out.Edges) != 1 {
		t.Fatalf("expected redraw to be idempotent, got %d edges", len(out.Edges))
	}
	if out.Edges[0].Why != "sharper reason" {
		t.Fatalf("expected why updated on redraw, got %q", out.Edges[0].Why)
	}
}

// TestEntityContext_MergeReview covers "Same, or just similar?" — the judge's open
// questions and the user's decisions on them. This is the review UI the judge sweep has
// been writing proposals for with nowhere to show them.
func TestEntityContext_MergeReview(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	userID := uuid.NewString()
	aName, bName, cName := "Mrga"+tag, "Mrgb"+tag, "Mrgc"+tag

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())`,
		userID, "u-"+tag+"@test.local"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var aID, bID, cID string
	for _, e := range []struct {
		name string
		into *string
	}{{aName, &aID}, {bName, &bID}, {cName, &cID}} {
		if err := pool.QueryRow(ctx, `
			INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier)
			VALUES ('shared', 'org', $1, $2, 'believed') RETURNING id::text
		`, e.name, entities.Normalize(e.name)).Scan(e.into); err != nil {
			t.Fatalf("seed entity: %v", err)
		}
	}
	// Two proposals on A: one the judge called "same", one it abstained on. Note the
	// second names A as the INTO side — it must still surface from A.
	if _, err := pool.Exec(ctx, `
		INSERT INTO entity_merges (from_entity_id, into_entity_id, status, confidence, method, evidence)
		VALUES ($1::uuid, $2::uuid, 'proposed', 0.78, 'placeholder_sweep',
		        '{"judge":{"verdict":"same","reason":"acronym of the same company"}}'::jsonb)
	`, aID, bID); err != nil {
		t.Fatalf("seed merge same: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO entity_merges (from_entity_id, into_entity_id, status, confidence, method, evidence)
		VALUES ($1::uuid, $2::uuid, 'proposed', 0.5, 'placeholder_sweep', '{}'::jsonb)
	`, cID, aID); err != nil {
		t.Fatalf("seed merge unjudged: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM entity_merges WHERE from_entity_id IN ($1::uuid,$2::uuid,$3::uuid) OR into_entity_id IN ($1::uuid,$2::uuid,$3::uuid)`, aID, bID, cID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1::uuid`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id IN ($1::uuid,$2::uuid,$3::uuid)`, aID, bID, cID)
	})

	svc := coreservice.NewEntityContextService(NewEntityContextPostgres(pool), db.NewEntityRepository(pool))

	out, err := svc.Get(ctx, userID, aID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(out.Merges) != 2 {
		t.Fatalf("expected both proposals (from- AND into-side), got %+v", out.Merges)
	}
	byOther := map[string]coreservice.PendingMerge{}
	for _, m := range out.Merges {
		byOther[m.OtherName] = m
	}
	// The judged one reads as a synonym suggestion, with its reasoning and evidence.
	same := byOther[bName]
	if same.Suggestion != coreservice.MergeSuggestSynonym {
		t.Fatalf("judge said same → expected synonym, got %q", same.Suggestion)
	}
	if same.Reason == "" || same.Why != "name overlap 78% · from an unresolved @mention" {
		t.Fatalf("expected the evidence rendered, got why=%q reason=%q", same.Why, same.Reason)
	}
	// The un-judged one is honestly "unsure" — never a guess.
	if byOther[cName].Suggestion != coreservice.MergeSuggestUnsure {
		t.Fatalf("un-judged pair must read unsure, got %q", byOther[cName].Suggestion)
	}

	// Reject → the pair is distinct; it leaves the queue and stays as negative evidence.
	if err := svc.RejectMerge(ctx, byOther[cName].MergeID); err != nil {
		t.Fatalf("reject: %v", err)
	}
	// Rejecting again is a conflict, not a silent success or a 500.
	if err := svc.RejectMerge(ctx, byOther[cName].MergeID); !errors.Is(err, coreservice.ErrMergeNotPending) {
		t.Fatalf("expected ErrMergeNotPending on re-decide, got %v", err)
	}

	// Accept → the entities fold together.
	if err := svc.AcceptMerge(ctx, same.MergeID); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if err := svc.AcceptMerge(ctx, same.MergeID); !errors.Is(err, coreservice.ErrMergeNotPending) {
		t.Fatalf("expected ErrMergeNotPending on double-accept, got %v", err)
	}

	// The queue is now empty, and A resolves to B (accept points from→into).
	out, err = svc.Get(ctx, userID, aID)
	if err != nil {
		t.Fatalf("get after decisions: %v", err)
	}
	if len(out.Merges) != 0 {
		t.Fatalf("expected no pending merges after deciding both, got %+v", out.Merges)
	}
	if out.Entity.ID != bID {
		t.Fatalf("after accept, A's page must resolve to B (%s), got %s", bID, out.Entity.ID)
	}
}

// TestEntityContext_MergeQueue covers the inbox: the global list of pending decisions,
// both sides named, with merged-away pairs excluded (stale questions the overlay
// already answered).
func TestEntityContext_MergeQueue(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	defer pool.Close()

	tag := uuid.NewString()[:8]
	live1, live2, staleFrom, root := "Q1"+tag, "Q2"+tag, "Qstale"+tag, "Qroot"+tag
	ids := map[string]string{}
	for _, n := range []string{live1, live2, staleFrom, root} {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO entities (scope, kind, canonical_name, normalized_key, trust_tier)
			VALUES ('shared', 'org', $1, $2, 'believed') RETURNING id::text
		`, n, entities.Normalize(n)).Scan(&id); err != nil {
			t.Fatalf("seed %s: %v", n, err)
		}
		ids[n] = id
	}
	// staleFrom is merged away → any proposal touching it must NOT surface.
	if _, err := pool.Exec(ctx, `UPDATE entities SET canonical_root_id = $2::uuid WHERE id = $1::uuid`,
		ids[staleFrom], ids[root]); err != nil {
		t.Fatalf("merge stale: %v", err)
	}
	mk := func(from, into, verdict string) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO entity_merges (from_entity_id, into_entity_id, status, confidence, method, evidence)
			VALUES ($1::uuid, $2::uuid, 'proposed', 0.8, 'placeholder_sweep',
			        CASE WHEN $3 = '' THEN '{}'::jsonb ELSE jsonb_build_object('judge', jsonb_build_object('verdict', $3)) END)
		`, ids[from], ids[into], verdict); err != nil {
			t.Fatalf("seed merge: %v", err)
		}
	}
	mk(live1, live2, "same")       // a live, judged proposal
	mk(staleFrom, root, "same")    // touches a merged-away entity → excluded
	t.Cleanup(func() {
		bg := context.Background()
		for _, id := range ids {
			_, _ = pool.Exec(bg, `DELETE FROM entity_merges WHERE from_entity_id = $1::uuid OR into_entity_id = $1::uuid`, id)
		}
		for _, id := range ids {
			_, _ = pool.Exec(bg, `DELETE FROM entities WHERE id = $1::uuid`, id)
		}
	})

	svc := coreservice.NewEntityContextService(NewEntityContextPostgres(pool), db.NewEntityRepository(pool))
	q, err := svc.MergeQueue(ctx, 200)
	if err != nil {
		t.Fatalf("merge queue: %v", err)
	}
	var mine *coreservice.MergeQueueItem
	for i := range q {
		if q[i].FromID == ids[live1] {
			mine = &q[i]
		}
		if q[i].FromID == ids[staleFrom] || q[i].IntoID == ids[staleFrom] {
			t.Fatalf("merged-away proposal leaked into the queue: %+v", q[i])
		}
	}
	if mine == nil {
		t.Fatalf("expected the live proposal in the queue, got %+v", q)
	}
	if mine.IntoName != live2 || mine.Suggestion != coreservice.MergeSuggestSynonym {
		t.Fatalf("queue item = %+v", *mine)
	}
}
