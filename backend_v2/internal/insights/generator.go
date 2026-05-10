package insights

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"aladin/backend_v2/internal/db"
)

const TopicTrendGeneratorKey = "topic_trend"

type Generator struct {
	insights         db.InsightRepository
	records          db.RecordRepository
	pool             *pgxpool.Pool
	recordGenerators map[string]RecordInsightGenerator
}

func NewGenerator(insights db.InsightRepository, records db.RecordRepository, pool *pgxpool.Pool) *Generator {
	g := &Generator{
		insights: insights,
		records:  records,
		pool:     pool,
	}
	g.recordGenerators = map[string]RecordInsightGenerator{
		TopicTrendGeneratorKey: topicTrendRecordGenerator{pool: pool},
	}
	return g
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

type RecordInsightInput struct {
	KgID           string
	RecordID       string
	SourceRevision int64
	CorrelationID  string
	GeneratorKeys  []string
	Record         *db.Record
}

type RecordInsightGenerator interface {
	Key() string
	Generate(ctx context.Context, input RecordInsightInput) ([]*db.Insight, error)
}

func (g *Generator) GenerateForRecord(ctx context.Context, input RecordInsightInput) (int, error) {
	record, err := g.records.Get(ctx, input.RecordID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	if input.SourceRevision != 0 && record.SourceRevision != input.SourceRevision {
		return 0, nil
	}
	input.Record = record

	var total int
	for _, key := range g.resolveRecordGeneratorKeys(input) {
		generator := g.recordGenerators[key]
		if generator == nil {
			slog.Warn("insights: unknown record generator", "component", "insights", "generator", key)
			continue
		}
		insights, err := generator.Generate(ctx, input)
		if err != nil {
			return total, fmt.Errorf("record insight generator %s: %w", key, err)
		}
		stored, err := g.store(ctx, input.KgID, insights)
		if err != nil {
			return total, err
		}
		total += stored
	}
	return total, nil
}

func (g *Generator) resolveRecordGeneratorKeys(input RecordInsightInput) []string {
	if len(input.GeneratorKeys) > 0 {
		return uniqueStrings(input.GeneratorKeys)
	}
	if input.Record != nil && input.Record.Type == "post" {
		return []string{TopicTrendGeneratorKey}
	}
	return nil
}

// ── Finders ───────────────────────────────────────────────────────────────────

// findTrendInsights finds topics appearing in 3+ records in the last 24 hours.
// Simple, no LLM — pure SQL over the enrichment JSONB.
func (g *Generator) findTrendInsights(ctx context.Context, kgID string) ([]*db.Insight, error) {
	rows, err := g.pool.Query(ctx, `
		SELECT
			topic,
			count(DISTINCT a.id)        AS mention_count,
			array_agg(DISTINCT a.id)    AS record_ids
		FROM records a
		JOIN tenant_item_matches tim ON tim.record_id = a.id
		JOIN source_subscriptions ss ON ss.id = tim.subscription_id
		CROSS JOIN LATERAL jsonb_array_elements_text(a.enrichment->'topics') AS topic
		WHERE ss.kg_id = $1::uuid
		  AND tim.relevance_status = 'relevant'
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

type topicTrendRecordGenerator struct {
	pool *pgxpool.Pool
}

func (g topicTrendRecordGenerator) Key() string { return TopicTrendGeneratorKey }

func (g topicTrendRecordGenerator) Generate(ctx context.Context, input RecordInsightInput) ([]*db.Insight, error) {
	if input.Record == nil || len(input.Record.Enrichment.Topics) == 0 {
		return nil, nil
	}
	topics, _ := json.Marshal(uniqueStrings(input.Record.Enrichment.Topics))
	rows, err := g.pool.Query(ctx, `
		WITH trigger_topics AS (
			SELECT jsonb_array_elements_text($2::jsonb) AS topic
		)
		SELECT
			topic.topic,
			count(DISTINCT a.id)        AS mention_count,
			array_agg(DISTINCT a.id)    AS record_ids
		FROM records a
		JOIN tenant_item_matches tim ON tim.record_id = a.id
		JOIN source_subscriptions ss ON ss.id = tim.subscription_id
		CROSS JOIN LATERAL jsonb_array_elements_text(a.enrichment->'topics') AS topic(topic)
		JOIN trigger_topics tt ON tt.topic = topic.topic
		WHERE ss.kg_id = $1::uuid
		  AND tim.relevance_status = 'relevant'
		  AND a.created_at >= now() - interval '24 hours'
		  AND a.enrichment IS NOT NULL
		GROUP BY topic.topic
		HAVING count(DISTINCT a.id) >= 3
		ORDER BY mention_count DESC
		LIMIT 10
	`, input.KgID, string(topics))
	if err != nil {
		return nil, fmt.Errorf("topic trend for record: %w", err)
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
			Body:       fmt.Sprintf("'%s' appeared in %d matched records in the last 24 hours.", topic, count),
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

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
