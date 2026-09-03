package storage

import (
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardv2"
	"sort"
	"testing"
	"time"
)

func TestShardResourceDefaultBoundLoad(t *testing.T) {
	h := setupResourceHarness(t, ShardResourceLimits{})
	// Fill the declared namespace to its default record cap, without timing test
	// setup or thousands of irrelevant receipt writes.
	_, err := h.pool.Exec(h.ctx, `INSERT INTO shard_resource_records(user_id,shard_id,environment,generation,dataset_id,id,schema_version,revision,data,data_bytes,created_by,updated_by)
 SELECT $1::uuid,$2,'published','generation-1','tasks','load-'||i,1,1,jsonb_build_object('title','Task '||i,'score',i),64,'load','load' FROM generate_series(1,10000) i`, testAdminUserID, h.target.ShardID)
	if err != nil {
		t.Fatal(err)
	}
	release, err := h.repo.ActiveResourceRelease(h.ctx, testAdminUserID, h.target.ShardID, h.target.Environment)
	if err != nil {
		t.Fatal(err)
	}
	// Use the fixture's actual generation (keeps this test independent of naming).
	_, err = h.pool.Exec(h.ctx, `UPDATE shard_resource_records SET generation=$1 WHERE shard_id=$2`, release.Generation, h.target.ShardID)
	if err != nil {
		t.Fatal(err)
	}
	request := service.ResourceRequest{Binding: "tasks", Query: &shardv2.Query{Limit: 500, OrderBy: []shardv2.Order{{Field: "/score", Direction: "desc"}}, Where: &shardv2.Predicate{Field: "/score", Op: "gte", Value: float64(0)}}}
	durations := []time.Duration{}
	for i := 0; i < 21; i++ {
		start := time.Now()
		result, err := h.svc.Read(h.ctx, h.target, request)
		if err != nil || len(result.Records) != 500 || result.NextCursor == "" {
			t.Fatalf("bounded query: %d %v", len(result.Records), err)
		}
		if i > 0 {
			durations = append(durations, time.Since(start))
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	median, p95 := durations[len(durations)/2], durations[len(durations)*95/100]
	t.Logf("10000 stored records; filter + numeric sort + 500 validated records; 20 reads: median=%s p95=%s", median, p95)
	// A staging admission budget, not a promised production SLO. The one-second
	// refresh profile cannot be admitted if a bounded query itself takes >1s.
	if p95 > time.Second {
		t.Fatalf("default refresh budget exceeded: %s", p95)
	}
}
