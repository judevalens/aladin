package repo

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// vec1536 builds a pgvector(1536) literal with the given leading values, zero-padded.
func vec1536(lead ...float32) string {
	parts := make([]string, 1536)
	for i := range parts {
		if i < len(lead) {
			parts[i] = strconv.FormatFloat(float64(lead[i]), 'f', -1, 32)
		} else {
			parts[i] = "0"
		}
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// TestSimilarRecords verifies the vector lens ranks a near-duplicate above an unrelated
// record by cosine similarity.
func TestSimilarRecords(t *testing.T) {
	ctx := context.Background()
	ctxTO, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool := mustTestPool(ctxTO, t)
	t.Cleanup(pool.Close) // registered first → runs last, after the row cleanup below

	a := "sim-a-" + uuid.NewString()
	near := "sim-near-" + uuid.NewString()
	far := "sim-far-" + uuid.NewString()

	seed := func(id, embed string) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO records (id, type, label, content, status, source_revision, embedding)
			VALUES ($1, 'post', $1, 'c', 'complete', 1, $2::vector)
		`, id, embed); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	seed(a, vec1536(1, 0, 0))
	seed(near, vec1536(0.95, 0.05, 0)) // almost parallel to a
	seed(far, vec1536(0, 1, 0))        // orthogonal to a
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM records WHERE id IN ($1, $2, $3)`, a, near, far)
	})

	sims, err := NewRecordPostgres(pool).SimilarRecords(ctx, a, 50)
	if err != nil {
		t.Fatalf("SimilarRecords: %v", err)
	}

	posNear, posFar, cosNear := -1, -1, 0.0
	for i, s := range sims {
		if s.ID == a {
			t.Fatal("the target record must not appear in its own similars")
		}
		switch s.ID {
		case near:
			posNear, cosNear = i, s.Cosine
		case far:
			posFar = i
		}
	}
	if posNear == -1 || posFar == -1 {
		t.Fatalf("expected both neighbours in results; near=%d far=%d (%+v)", posNear, posFar, sims)
	}
	// The near-duplicate must rank above the orthogonal record, with high cosine.
	if posNear >= posFar {
		t.Fatalf("near-duplicate ranked at %d, not above the orthogonal record at %d", posNear, posFar)
	}
	if cosNear < 0.9 {
		t.Fatalf("near-duplicate cosine = %f, want > 0.9", cosNear)
	}
}
