package entities

import (
	"context"
	"strings"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"

	"github.com/google/uuid"
)

// TestResolver_AgainstPostgres exercises the full R0 slice against the sandbox DB:
// the 00010 migration must apply, the repo must round-trip, and the resolver must
// collapse surface variants of one entity while keeping distinct names distinct.
func TestResolver_AgainstPostgres(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test db (00010 entity layer must apply): %v", err)
	}

	recordID := "entity-res-test-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO records (id, type, label, content, status, source_revision)
		VALUES ($1, 'story', 'test', 'content', 'enriched', 1)
		ON CONFLICT (id) DO NOTHING
	`, recordID); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	t.Cleanup(func() {
		// entity_mentions cascade on the record delete; the shared entities created
		// here are global, so clean them by normalized key to keep the sandbox tidy.
		_, _ = pool.Exec(context.Background(), `DELETE FROM records WHERE id = $1`, recordID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE scope='shared' AND normalized_key IN ('openai','anthropic')`)
	})

	r := NewResolver(db.NewEntityRepository(pool))

	ids := map[string]bool{}
	for _, surface := range []string{"OpenAI", "OpenAI Inc", "OpenAI, Inc.", "openai"} {
		id, err := r.Resolve(ctx, Mention{Surface: surface, RecordID: recordID, SourceRevision: 1})
		if err != nil {
			t.Fatalf("resolve %q: %v", surface, err)
		}
		if id == "" {
			t.Fatalf("resolve %q returned empty id", surface)
		}
		ids[id] = true
	}
	if len(ids) != 1 {
		t.Fatalf("expected OpenAI variants to collapse to 1 entity, got %d", len(ids))
	}

	other, err := r.Resolve(ctx, Mention{Surface: "Anthropic", RecordID: recordID, SourceRevision: 1})
	if err != nil {
		t.Fatalf("resolve Anthropic: %v", err)
	}
	if ids[other] {
		t.Fatalf("distinct name resolved into the same entity")
	}

	// Re-resolving an already-seen surface must not duplicate the mention.
	if _, err := r.Resolve(ctx, Mention{Surface: "OpenAI", RecordID: recordID, SourceRevision: 1}); err != nil {
		t.Fatalf("re-resolve OpenAI: %v", err)
	}
	var mentions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM entity_mentions WHERE record_id = $1`, recordID).Scan(&mentions); err != nil {
		t.Fatalf("count mentions: %v", err)
	}
	// 4 distinct OpenAI surfaces + 1 Anthropic = 5; the re-resolve is a no-op.
	if mentions != 5 {
		t.Fatalf("expected 5 mentions (4 OpenAI surfaces + Anthropic, re-resolve idempotent), got %d", mentions)
	}
}

// TestResolver_FuzzyProposesMerge_AndNegativeEvidence: a near-match (one trailing char)
// is NOT auto-merged — it lands as a distinct entity plus a proposed merge — and once a
// human rejects that merge, the pair is never re-proposed (negative evidence).
func TestResolver_FuzzyProposesMerge_AndNegativeEvidence(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tag := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	nameA := "novacorp" + tag      // normalized key == itself (lowercase, no suffix)
	nameB := nameA + "x"           // one trailing char → high trigram similarity, distinct key
	recordID := "entity-fuzzy-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO records (id, type, label, content, status, source_revision)
		VALUES ($1, 'story', 'test', 'content', 'enriched', 1)
		ON CONFLICT (id) DO NOTHING
	`, recordID); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM records WHERE id = $1`, recordID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE scope='shared' AND normalized_key IN ($1, $2)`, nameA, nameB)
	})

	repo := db.NewEntityRepository(pool)
	r := NewResolver(repo)

	idA, err := r.Resolve(ctx, Mention{Surface: nameA, RecordID: recordID, SourceRevision: 1})
	if err != nil {
		t.Fatalf("resolve A: %v", err)
	}
	idB, err := r.Resolve(ctx, Mention{Surface: nameB, RecordID: recordID, SourceRevision: 1})
	if err != nil {
		t.Fatalf("resolve B: %v", err)
	}
	if idA == "" || idB == "" || idA == idB {
		t.Fatalf("fuzzy match must stay distinct (not auto-merged): A=%q B=%q", idA, idB)
	}

	proposals, err := repo.ListProposedMerges(ctx, 100)
	if err != nil {
		t.Fatalf("list proposed: %v", err)
	}
	var mergeID string
	for _, p := range proposals {
		if (p.FromEntityID == idB && p.IntoEntityID == idA) || (p.FromEntityID == idA && p.IntoEntityID == idB) {
			mergeID = p.ID
		}
	}
	if mergeID == "" {
		t.Fatalf("expected a proposed merge for the fuzzy pair %q ~ %q", nameA, nameB)
	}

	if err := repo.RejectMerge(ctx, mergeID); err != nil {
		t.Fatalf("reject: %v", err)
	}
	reproposed, err := repo.ProposeMerge(ctx, db.ProposeMergeParams{
		FromEntityID: idB, IntoEntityID: idA, Confidence: 0.9, Method: "trigram",
	})
	if err != nil {
		t.Fatalf("re-propose: %v", err)
	}
	if reproposed {
		t.Fatal("a rejected pair must not be re-proposed (negative evidence)")
	}
}

// TestEntityRepo_AcceptMergeSetsCanonicalRoot: accepting a proposed merge points the
// from-entity's canonical_root_id at the into-entity (the reversible overlay write).
func TestEntityRepo_AcceptMergeSetsCanonicalRoot(t *testing.T) {
	dsn := dbtest.RequireTestDSN(t)
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tag := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	keyInto := "acceptinto" + tag
	keyFrom := "acceptfrom" + tag
	repo := db.NewEntityRepository(pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE scope='shared' AND normalized_key IN ($1, $2)`, keyInto, keyFrom)
	})

	into, err := repo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: keyInto, NormalizedKey: keyInto})
	if err != nil {
		t.Fatalf("create into: %v", err)
	}
	from, err := repo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: keyFrom, NormalizedKey: keyFrom})
	if err != nil {
		t.Fatalf("create from: %v", err)
	}
	if _, err := repo.ProposeMerge(ctx, db.ProposeMergeParams{FromEntityID: from, IntoEntityID: into, Confidence: 0.7, Method: "manual"}); err != nil {
		t.Fatalf("propose: %v", err)
	}

	proposals, err := repo.ListProposedMerges(ctx, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var mergeID string
	for _, p := range proposals {
		if p.FromEntityID == from && p.IntoEntityID == into {
			mergeID = p.ID
		}
	}
	if mergeID == "" {
		t.Fatal("expected the proposed merge to be listed")
	}
	if err := repo.AcceptMerge(ctx, mergeID); err != nil {
		t.Fatalf("accept: %v", err)
	}

	var root string
	if err := pool.QueryRow(ctx, `SELECT canonical_root_id::text FROM entities WHERE id = $1::uuid`, from).Scan(&root); err != nil {
		t.Fatalf("query root: %v", err)
	}
	if root != into {
		t.Fatalf("expected from.canonical_root_id = into (%s), got %s", into, root)
	}
}

// TestResolver_TenantTier_BindAndIsolation (R3): a tenant mention of a known shared
// entity creates a per-tenant overlay *bound* to the shared canonical; two tenants get
// distinct overlays (isolation); and ResolveCanonicalRoot crosses the bind to the shared
// id for both.
func TestResolver_TenantTier_BindAndIsolation(t *testing.T) {
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

	tag := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	key := "acme" + tag
	ownerA := uuid.NewString()
	ownerB := uuid.NewString()
	recordID := "entity-tenant-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO records (id, type, label, content, status, source_revision)
		VALUES ($1, 'story', 'test', 'content', 'enriched', 1) ON CONFLICT (id) DO NOTHING
	`, recordID); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM records WHERE id = $1`, recordID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE normalized_key = $1`, key)
	})

	repo := db.NewEntityRepository(pool)
	r := NewResolver(repo)

	shared, err := repo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: key, NormalizedKey: key})
	if err != nil {
		t.Fatalf("create shared: %v", err)
	}

	tA, err := r.ResolveTenant(ctx, ownerA, Mention{Surface: key, RecordID: recordID, SourceRevision: 1})
	if err != nil {
		t.Fatalf("resolve tenant A: %v", err)
	}
	tA2, err := r.ResolveTenant(ctx, ownerA, Mention{Surface: key, RecordID: recordID, SourceRevision: 1})
	if err != nil {
		t.Fatalf("resolve tenant A again: %v", err)
	}
	tB, err := r.ResolveTenant(ctx, ownerB, Mention{Surface: key, RecordID: recordID, SourceRevision: 1})
	if err != nil {
		t.Fatalf("resolve tenant B: %v", err)
	}

	if tA != tA2 {
		t.Fatalf("tenant overlay must be idempotent for the same owner: %s vs %s", tA, tA2)
	}
	if tA == tB {
		t.Fatal("cross-tenant isolation: distinct owners must get distinct entities")
	}
	if tA == shared || tB == shared {
		t.Fatal("a tenant overlay must be a distinct entity from the shared canonical")
	}

	rootA, err := repo.ResolveCanonicalRoot(ctx, tA)
	if err != nil {
		t.Fatalf("resolve root A: %v", err)
	}
	rootB, err := repo.ResolveCanonicalRoot(ctx, tB)
	if err != nil {
		t.Fatalf("resolve root B: %v", err)
	}
	if rootA != shared || rootB != shared {
		t.Fatalf("both tenant overlays should resolve through their bind to the shared canonical %s; got A=%s B=%s", shared, rootA, rootB)
	}
}

// TestResolver_TenantTier_LocalOverride (R3): a tenant's own merge wins over its bind to
// the shared tier — after merging a bound overlay into another tenant entity, the
// canonical root is the tenant target, not the shared canonical.
func TestResolver_TenantTier_LocalOverride(t *testing.T) {
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

	tag := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	keyBound := "globex" + tag
	keyOther := "globexalt" + tag
	owner := uuid.NewString()
	recordID := "entity-override-" + uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO records (id, type, label, content, status, source_revision)
		VALUES ($1, 'story', 'test', 'content', 'enriched', 1) ON CONFLICT (id) DO NOTHING
	`, recordID); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM records WHERE id = $1`, recordID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE normalized_key IN ($1, $2)`, keyBound, keyOther)
	})

	repo := db.NewEntityRepository(pool)
	r := NewResolver(repo)

	shared, err := repo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: keyBound, NormalizedKey: keyBound})
	if err != nil {
		t.Fatalf("create shared: %v", err)
	}
	bound, err := r.ResolveTenant(ctx, owner, Mention{Surface: keyBound, RecordID: recordID, SourceRevision: 1}) // binds to shared
	if err != nil {
		t.Fatalf("resolve bound: %v", err)
	}
	other, err := r.ResolveTenant(ctx, owner, Mention{Surface: keyOther, RecordID: recordID, SourceRevision: 1}) // unbound (no shared match)
	if err != nil {
		t.Fatalf("resolve other: %v", err)
	}

	// Tenant-local merge: bound -> other.
	if _, err := repo.ProposeMerge(ctx, db.ProposeMergeParams{FromEntityID: bound, IntoEntityID: other, Confidence: 1, Method: "manual"}); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposals, err := repo.ListProposedMerges(ctx, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var mergeID string
	for _, p := range proposals {
		if p.FromEntityID == bound && p.IntoEntityID == other {
			mergeID = p.ID
		}
	}
	if mergeID == "" {
		t.Fatal("expected the tenant-local proposed merge")
	}
	if err := repo.AcceptMerge(ctx, mergeID); err != nil {
		t.Fatalf("accept: %v", err)
	}

	root, err := repo.ResolveCanonicalRoot(ctx, bound)
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	if root != other {
		t.Fatalf("local override: the merged overlay should resolve to the tenant target %s, not the shared canonical %s; got %s", other, shared, root)
	}
}

// TestEntityRepo_RevertMergePopsEntityBack (R4): accepting then reverting a merge
// returns the entity to itself — the reversibility guarantee.
func TestEntityRepo_RevertMergePopsEntityBack(t *testing.T) {
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

	tag := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	keyInto := "revinto" + tag
	keyFrom := "revfrom" + tag
	repo := db.NewEntityRepository(pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE scope='shared' AND normalized_key IN ($1, $2)`, keyInto, keyFrom)
	})

	into, err := repo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: keyInto, NormalizedKey: keyInto})
	if err != nil {
		t.Fatalf("create into: %v", err)
	}
	from, err := repo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: keyFrom, NormalizedKey: keyFrom})
	if err != nil {
		t.Fatalf("create from: %v", err)
	}
	if _, err := repo.ProposeMerge(ctx, db.ProposeMergeParams{FromEntityID: from, IntoEntityID: into, Confidence: 1, Method: "manual"}); err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposals, err := repo.ListProposedMerges(ctx, 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var mergeID string
	for _, p := range proposals {
		if p.FromEntityID == from && p.IntoEntityID == into {
			mergeID = p.ID
		}
	}
	if mergeID == "" {
		t.Fatal("expected the proposed merge")
	}

	if err := repo.AcceptMerge(ctx, mergeID); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if root, err := repo.ResolveCanonicalRoot(ctx, from); err != nil || root != into {
		t.Fatalf("after accept, from should resolve to into (%s); got %s err=%v", into, root, err)
	}

	if err := repo.RevertMerge(ctx, mergeID); err != nil {
		t.Fatalf("revert: %v", err)
	}
	if root, err := repo.ResolveCanonicalRoot(ctx, from); err != nil || root != from {
		t.Fatalf("after revert, from should pop back to itself (%s); got %s err=%v", from, root, err)
	}
}

// unitVec builds a 1536-dim one-hot vector (matches the entities.embedding column).
func unitVec(i int) []float32 {
	v := make([]float32, 1536)
	v[i] = 1
	return v
}

// TestEntityRepo_EmbeddingStoreAndVectorSearch (R2): the pgvector plumbing — store an
// entity embedding, read it back, and find cosine-similar entities while excluding
// orthogonal ones via the threshold.
func TestEntityRepo_EmbeddingStoreAndVectorSearch(t *testing.T) {
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

	tag := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	keyNear := "vnear" + tag
	keyFar := "vfar" + tag
	repo := db.NewEntityRepository(pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM entities WHERE scope='shared' AND normalized_key IN ($1, $2)`, keyNear, keyFar)
	})

	near, err := repo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: keyNear, NormalizedKey: keyNear})
	if err != nil {
		t.Fatalf("create near: %v", err)
	}
	far, err := repo.CreateSharedEntity(ctx, db.CreateEntityParams{Kind: "unknown", CanonicalName: keyFar, NormalizedKey: keyFar})
	if err != nil {
		t.Fatalf("create far: %v", err)
	}
	if err := repo.SetEntityEmbedding(ctx, near, unitVec(5)); err != nil {
		t.Fatalf("set near embedding: %v", err)
	}
	if err := repo.SetEntityEmbedding(ctx, far, unitVec(900)); err != nil {
		t.Fatalf("set far embedding: %v", err)
	}

	got, has, err := repo.GetEntityEmbedding(ctx, near)
	if err != nil || !has {
		t.Fatalf("get embedding: has=%v err=%v", has, err)
	}
	if len(got) != 1536 || got[5] < 0.99 {
		t.Fatalf("embedding roundtrip mismatch: len=%d got[5]=%v", len(got), got[5])
	}

	cands, err := repo.FindSharedCandidatesByVector(ctx, "unknown", "noexclude", unitVec(5), 0.75, 50)
	if err != nil {
		t.Fatalf("vector search: %v", err)
	}
	var foundNear, foundFar bool
	for _, c := range cands {
		if c.ID == near {
			foundNear = true
		}
		if c.ID == far {
			foundFar = true
		}
	}
	if !foundNear {
		t.Fatal("the cosine-similar entity must be returned")
	}
	if foundFar {
		t.Fatal("the orthogonal entity must be filtered out by the cosine threshold")
	}
}
