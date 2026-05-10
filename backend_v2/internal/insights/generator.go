package insights

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"aladin/backend_v2/internal/db"
)

type Generator struct {
	insights db.InsightRepository
	pool     *pgxpool.Pool
}

func NewGenerator(insights db.InsightRepository, pool *pgxpool.Pool) *Generator {
	return &Generator{insights: insights, pool: pool}
}

// GenerateAndStore runs all finders for a KG and persists new insights.
// Returns the number of new insights stored.
func (g *Generator) GenerateAndStore(ctx context.Context, kgID string) (int, error) {
	var total int

	finders := []func(context.Context, string) ([]*db.Insight, error){
		g.findTrendInsights,
	}

	for _, find := range finders {
		insights, err := find(ctx, kgID)
		if err != nil {
			slog.Error("insights: finder failed", "component", "insights", "kg_id", kgID, "err", err)
			continue
		}
		n, err := g.store(ctx, kgID, insights)
		if err != nil {
			slog.Error("insights: store failed", "component", "insights", "kg_id", kgID, "err", err)
			continue
		}
		total += n
	}

	return total, nil
}

// ── Finders ───────────────────────────────────────────────────────────────────

// findTrendInsights finds topics appearing in 3+ records in the last 24 hours.
// Simple, no LLM — pure SQL over the enrichment JSONB.
func (g *Generator) findTrendInsights(ctx context.Context, kgID string) ([]*db.Insight, error) {
	rows, err := g.pool.Query(ctx, `
		SELECT
			topic,
			count(*)        AS mention_count,
			array_agg(a.id) AS record_ids
		FROM records a
		JOIN tenant_item_matches tim ON tim.record_id = a.id
		JOIN source_subscriptions ss ON ss.id = tim.subscription_id
		CROSS JOIN LATERAL jsonb_array_elements_text(a.enrichment->'topics') AS topic
		WHERE ss.kg_id = $1::uuid
		  AND a.created_at >= now() - interval '24 hours'
		  AND a.enrichment IS NOT NULL
		GROUP BY topic
		HAVING count(*) >= 3
		ORDER BY mention_count DESC
		LIMIT 10
	`, kgID)
	if err != nil {
		return nil, fmt.Errorf("findTrendInsights: %w", err)
	}
	defer rows.Close()

	var insights []*db.Insight
	for rows.Next() {
		var topic string
		var count int
		var recordIDs []string
		if err := rows.Scan(&topic, &count, &recordIDs); err != nil {
			return nil, err
		}
		insights = append(insights, &db.Insight{
			Type:       "trend",
			Title:      fmt.Sprintf("'%s' is trending", topic),
			Body:       fmt.Sprintf("'%s' appeared in %d articles in the last 24 hours.", topic, count),
			Topic:      topic,
			RecordIDs:  recordIDs,
			Confidence: min(0.5+float64(count)*0.05, 1.0),
		})
	}
	return insights, rows.Err()
}

// ── Storage ───────────────────────────────────────────────────────────────────

func (g *Generator) store(ctx context.Context, kgID string, insights []*db.Insight) (int, error) {
	stored := 0
	for _, ins := range insights {
		key := ins.Entity
		if key == "" {
			key = ins.Topic
		}
		exists, err := g.insights.ExistsRecent(ctx, kgID, ins.Type, key, ins.Title)
		if err != nil || exists {
			continue
		}
		if err := g.insights.Store(ctx, kgID, ins); err != nil {
			return stored, err
		}
		stored++
	}
	return stored, nil
}
