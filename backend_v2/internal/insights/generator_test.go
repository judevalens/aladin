package insights

import (
	"context"
	"testing"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/dbtest"

	"github.com/google/uuid"
)

// TestFindBridgeInsights seeds two relevant records that share an entity across
// two distinct topics and asserts the bridge finder surfaces that entity (and
// only it — single-topic entities must not appear).
func TestFindBridgeInsights(t *testing.T) {
	t.Parallel()
	dsn := dbtest.RequireTestDSN(t)

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	userID := uuid.NewString()
	kgID := uuid.NewString()
	streamID := uuid.NewString()
	subID := uuid.NewString()
	rec1 := "bridge-rec1-" + uuid.NewString()
	rec2 := "bridge-rec2-" + uuid.NewString()

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM tenant_item_matches WHERE subscription_id=$1`, subID)
		_, _ = pool.Exec(bg, `DELETE FROM records WHERE id = ANY($1)`, []string{rec1, rec2})
		_, _ = pool.Exec(bg, `DELETE FROM source_subscriptions WHERE id=$1`, subID)
		_, _ = pool.Exec(bg, `DELETE FROM insights WHERE kg_id=$1`, kgID)
		_, _ = pool.Exec(bg, `DELETE FROM provider_streams WHERE id=$1`, streamID)
		_, _ = pool.Exec(bg, `DELETE FROM knowledge_graphs WHERE id=$1`, kgID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id=$1`, userID)
	})

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%s): %v", sql, err)
		}
	}
	exec(`INSERT INTO users(id,email,created_at) VALUES($1,$2,now())`, userID, "bridge-"+userID+"@test.local")
	exec(`INSERT INTO knowledge_graphs(id,user_id,name) VALUES($1,$2,'bridge-kg')`, kgID, userID)
	exec(`INSERT INTO provider_streams(id,provider,stream_kind,stream_key,name) VALUES($1,'test','feed',$2,'s')`, streamID, "k-"+streamID)
	exec(`INSERT INTO source_subscriptions(id,user_id,kg_id,provider_stream_id,name) VALUES($1,$2,$3,$4,'sub')`, subID, userID, kgID, streamID)
	// rec1: Acme appears with topic Alpha; rec2: Acme with topic Beta → Acme bridges
	// two topics. Foo (Alpha only) and Bar (Beta only) are single-topic.
	exec(`INSERT INTO records(id,type,label,content,enrichment) VALUES($1,'post','l','c',$2::jsonb)`, rec1, `{"entities":["Acme","Foo"],"topics":["Alpha"]}`)
	exec(`INSERT INTO records(id,type,label,content,enrichment) VALUES($1,'post','l','c',$2::jsonb)`, rec2, `{"entities":["Acme","Bar"],"topics":["Beta"]}`)
	exec(`INSERT INTO tenant_item_matches(id,subscription_id,source_revision,match_source,record_id,relevance_status) VALUES($1,$2,0,'policy',$3,'relevant')`, uuid.NewString(), subID, rec1)
	exec(`INSERT INTO tenant_item_matches(id,subscription_id,source_revision,match_source,record_id,relevance_status) VALUES($1,$2,0,'policy',$3,'relevant')`, uuid.NewString(), subID, rec2)

	g := &Generator{pool: pool}
	got, err := g.findBridgeInsights(ctx, kgID)
	if err != nil {
		t.Fatalf("findBridgeInsights: %v", err)
	}

	var acme *db.Insight
	for _, ins := range got {
		if ins.Entity == "Foo" || ins.Entity == "Bar" {
			t.Fatalf("single-topic entity %q was wrongly surfaced as a bridge", ins.Entity)
		}
		if ins.Entity == "Acme" {
			acme = ins
		}
	}
	if acme == nil {
		t.Fatalf("expected a bridge insight for 'Acme'; got %d insight(s): %+v", len(got), got)
	}
	if acme.Type != "bridge" {
		t.Fatalf("want type=bridge, got %q", acme.Type)
	}
	if len(acme.RecordIDs) != 2 {
		t.Fatalf("want 2 supporting record ids, got %v", acme.RecordIDs)
	}
}
