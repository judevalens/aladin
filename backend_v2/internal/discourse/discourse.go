// Package discourse runs the bridge discourse pass — the ambient insight engine. A
// "bridge" is a shared entity connected to >= MinBridgeDegree documents (sourced from the
// resolved entity layer, entity_mentions, keyed on entity_id); an LLM judges each
// document's stance toward the entity; the result is stored as a grounded, versioned
// discourse map. Runs as a batched sweep over "due" bridges (grown enough since last
// analysis) so cost is bounded. This is buck's engine reworked off Neo4j/canonical-text
// onto the entity layer (~/.claude/plans/moat-consolidation.md).
package discourse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/ratelimit"
)

const (
	// MinBridgeDegree: an entity needs at least this many connected docs to be a bridge.
	MinBridgeDegree = 2
	// GrowthThreshold: re-analyze once a bridge gains this many connected docs.
	GrowthThreshold = 2
	// MaxMembers caps how many documents feed one discourse pass (cost + prompt size).
	MaxMembers = 25
)

// Source is the entity-layer-backed bridge source + ledger + store (db.DiscourseRepository
// satisfies it).
type Source interface {
	CandidateBridges(ctx context.Context, minDegree, limit int) ([]db.Bridge, error)
	BridgeMembers(ctx context.Context, entityID string, limit int) ([]db.BridgeMember, error)
	GetDiscourseLedger(ctx context.Context, entityID string) (*db.EntityDiscourse, error)
	MarkAnalyzed(ctx context.Context, entityID string, degree int) (int, error)
	StoreDiscourse(ctx context.Context, d *db.DiscourseInsight) error
}

// Service runs the discourse sweep.
type Service struct {
	src     Source
	judge   llm.DiscourseJudge
	limiter *ratelimit.Limiter
	now     func() time.Time
}

func NewService(src Source, judge llm.DiscourseJudge, limiter *ratelimit.Limiter) *Service {
	return &Service{src: src, judge: judge, limiter: limiter, now: time.Now}
}

// Sweep analyzes up to limit due bridges. Returns the number of discourse maps written.
func (s *Service) Sweep(ctx context.Context, limit int) (int, error) {
	bridges, err := s.src.CandidateBridges(ctx, MinBridgeDegree, limit)
	if err != nil {
		return 0, fmt.Errorf("discourse sweep: candidates: %w", err)
	}
	n := 0
	for _, b := range bridges {
		state, err := s.src.GetDiscourseLedger(ctx, b.EntityID)
		if err != nil {
			slog.Error("discourse: ledger get failed", "component", "discourse", "entity_id", b.EntityID, "err", err)
			continue
		}
		if !state.Due(b.Degree, GrowthThreshold) {
			continue
		}
		stored, err := s.AnalyzeBridge(ctx, b.EntityID, b.EntityName, b.Degree)
		if err != nil {
			slog.Error("discourse: analyze failed", "component", "discourse", "entity_id", b.EntityID, "err", err)
			continue
		}
		if stored {
			n++
		}
	}
	return n, nil
}

// AnalyzeBridge gathers an entity's connected members, runs the discourse pass, and (if
// any position grounds to a real member) stores a grounded map + records the analysis.
// Returns whether a map was stored.
func (s *Service) AnalyzeBridge(ctx context.Context, entityID, entityName string, degree int) (bool, error) {
	members, err := s.src.BridgeMembers(ctx, entityID, MaxMembers)
	if err != nil {
		return false, fmt.Errorf("bridge members: %w", err)
	}
	if len(members) < MinBridgeDegree {
		return false, nil // shrank below threshold between candidate scan and now
	}

	input := make([]llm.DiscourseMember, 0, len(members))
	byID := make(map[string]db.BridgeMember, len(members))
	for _, m := range members {
		text := m.Summary
		if strings.TrimSpace(text) == "" {
			text = m.Label
		}
		input = append(input, llm.DiscourseMember{ID: m.ID, Kind: m.Kind, Summary: text})
		byID[m.ID] = m
	}

	if s.limiter != nil {
		if err := s.limiter.Wait(ctx); err != nil {
			return false, err
		}
	}
	result, err := s.judge.JudgeDiscourse(ctx, entityName, input)
	if err != nil {
		return false, fmt.Errorf("judge: %w", err)
	}

	// Grounding: keep only positions citing a real member id, attach its source for
	// verifiability. A hallucinated member_id is dropped.
	type groundedPosition struct {
		MemberID  string `json:"member_id"`
		Kind      string `json:"kind"`
		Stance    string `json:"stance"`
		Claim     string `json:"claim"`
		SourceURL string `json:"source_url"`
	}
	grounded := make([]groundedPosition, 0, len(result.Positions))
	memberIDs := make([]string, 0, len(result.Positions))
	for _, p := range result.Positions {
		m, ok := byID[p.MemberID]
		if !ok {
			continue // ungrounded — drop
		}
		grounded = append(grounded, groundedPosition{
			MemberID: p.MemberID, Kind: m.Kind, Stance: p.Stance, Claim: p.Claim, SourceURL: m.SourceURL,
		})
		memberIDs = append(memberIDs, p.MemberID)
	}
	if len(grounded) == 0 {
		return false, nil // nothing verifiable — don't store noise
	}

	version, err := s.src.MarkAnalyzed(ctx, entityID, degree)
	if err != nil {
		return false, fmt.Errorf("mark analyzed: %w", err)
	}

	provenance, _ := json.Marshal(map[string]any{
		"positions":    grounded,
		"overall":      result.Overall,
		"model":        "gpt-4o-mini",
		"generated_at": s.now().UTC().Format(time.RFC3339),
		"member_count": len(members),
	})

	if err := s.src.StoreDiscourse(ctx, &db.DiscourseInsight{
		EntityID:   entityID,
		EntityName: entityName,
		Title:      result.Headline,
		Body:       result.Headline,
		MemberIDs:  memberIDs,
		Provenance: provenance,
		Confidence: result.Confidence,
		TrustTier:  "believed",
		Version:    version,
	}); err != nil {
		return false, fmt.Errorf("store discourse: %w", err)
	}

	slog.Info("discourse: map stored",
		"component", "discourse", "entity_id", entityID,
		"overall", result.Overall, "positions", len(grounded), "version", version,
	)
	return true, nil
}
