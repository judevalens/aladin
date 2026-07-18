package entities

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/search"
)

const (
	// judgeAutoMergeMinSim: at or above this trigram similarity a proposed pair is
	// applied deterministically (no LLM) — but ONLY when kinds are compatible.
	// Cross-typed-kind same-name pairs ("Apple" org vs "Apple" concept) are legitimate
	// sense splits and always go to the judge.
	judgeAutoMergeMinSim = 0.90
	// judgeCandidateLimit: how many candidates a placeholder proposes against.
	judgeCandidateLimit = 3
	// judgeSearchResults: web snippets per side on escalation.
	judgeSearchResults = 3
)

// JudgeStore is the narrow persistence surface the judge sweep needs; satisfied by
// db.EntityRepository. Kept narrow so tests fake only this.
type JudgeStore interface {
	ListPlaceholders(ctx context.Context, limit int) ([]db.PlaceholderEntity, error)
	FindMergeCandidatesAnyKind(ctx context.Context, entityID, normalizedKey string, minSim float64, limit int) ([]db.ScoredCandidate, error)
	ProposeMerge(ctx context.Context, p db.ProposeMergeParams) (bool, error)
	ListJudgeableMerges(ctx context.Context, limit int) ([]db.JudgeableMerge, error)
	DecideMerge(ctx context.Context, mergeID, outcome, decidedBy string, evidence map[string]any) error
	HasProposedMergeFor(ctx context.Context, entityID string) (bool, error)
	PromoteEntity(ctx context.Context, entityID string) error
	ListEntityAliases(ctx context.Context, entityID string) ([]string, error)
	ResolveCanonicalRoot(ctx context.Context, entityID string) (string, error)
}

// JudgeSweeper resolves the ambiguous middle of entity resolution in the background
// (P1.3): placeholders minted by the editor's @ create path, and proposed merges the
// deterministic signals couldn't settle. Escalation ladder per sweep item:
//
//	trigram ≥ 0.90 + compatible kinds → auto-apply (no LLM)
//	else → LLM judge on registry data → same/different → apply/reject
//	judge unsure + searcher configured → web-search both names, re-judge ONCE
//	still unsure → stays proposed (visible for manual review, never re-judged)
//
// Placeholders only GENERATE proposals; the band judging consumes them — one code path
// decides all merges. A placeholder with no candidates (or all pairs rejected) is
// promoted in place: it is a real, distinct entity. Nil judge and nil searcher are both
// fine — each tier degrades to the one below.
type JudgeSweeper struct {
	store    JudgeStore
	judge    llm.EntityAdjudicator // nil → deterministic tier only
	searcher search.Searcher       // nil → no web escalation
}

func NewJudgeSweeper(store JudgeStore, judge llm.EntityAdjudicator) *JudgeSweeper {
	return &JudgeSweeper{store: store, judge: judge}
}

// WithSearcher enables the web-search escalation for unsure verdicts.
func (s *JudgeSweeper) WithSearcher(sr search.Searcher) *JudgeSweeper {
	s.searcher = sr
	return s
}

// Sweep runs one pass: placeholder→proposals, band judging, placeholder finalize.
// Per-item failures are logged and skipped (the sweep is periodic; next pass retries).
// Returns how many items it acted on (proposals created, merges decided, promotions).
func (s *JudgeSweeper) Sweep(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	acted := 0

	// A. Placeholders generate proposals against their candidates (any kind,
	// alias-aware, exact keys included). No candidates at all → promote immediately.
	placeholders, err := s.store.ListPlaceholders(ctx, limit)
	if err != nil {
		return acted, fmt.Errorf("judge sweep: list placeholders: %w", err)
	}
	for _, ph := range placeholders {
		cands, err := s.store.FindMergeCandidatesAnyKind(ctx, ph.ID, ph.NormalizedKey, fuzzyProposeMinSim, judgeCandidateLimit)
		if err != nil {
			slog.Warn("judge sweep: candidates failed", "component", "entities", "entity", ph.ID, "err", err)
			continue
		}
		if len(cands) == 0 {
			if err := s.store.PromoteEntity(ctx, ph.ID); err != nil {
				slog.Warn("judge sweep: promote failed", "component", "entities", "entity", ph.ID, "err", err)
				continue
			}
			acted++
			continue
		}
		for _, c := range cands {
			inserted, err := s.store.ProposeMerge(ctx, db.ProposeMergeParams{
				FromEntityID: ph.ID,
				IntoEntityID: c.ID,
				Confidence:   c.Similarity,
				Method:       "placeholder_sweep",
				Evidence:     map[string]any{"placeholder": ph.CanonicalName, "candidate": c.CanonicalName, "sim": c.Similarity},
			})
			if err != nil {
				slog.Warn("judge sweep: propose failed", "component", "entities", "from", ph.ID, "into", c.ID, "err", err)
				continue
			}
			if inserted {
				acted++
			}
		}
	}

	// B. Judge the band: every un-judged proposal, whatever produced it.
	merges, err := s.store.ListJudgeableMerges(ctx, limit)
	if err != nil {
		return acted, fmt.Errorf("judge sweep: list merges: %w", err)
	}
	for _, m := range merges {
		outcome, decidedBy, evidence, err := s.decidePair(ctx, m)
		if err != nil {
			slog.Warn("judge sweep: judging failed", "component", "entities", "merge", m.ID, "err", err)
			continue
		}
		if outcome == "" {
			continue // nothing to decide with (no judge configured, below auto tier)
		}
		if err := s.store.DecideMerge(ctx, m.ID, outcome, decidedBy, evidence); err != nil {
			slog.Warn("judge sweep: decide failed", "component", "entities", "merge", m.ID, "err", err)
			continue
		}
		acted++
	}

	// C. Finalize placeholders: merged away → resolved; all pairs settled and it
	// survived → a real, distinct entity → promote; proposals pending → wait.
	for _, ph := range placeholders {
		root, err := s.store.ResolveCanonicalRoot(ctx, ph.ID)
		if err != nil {
			slog.Warn("judge sweep: resolve root failed", "component", "entities", "entity", ph.ID, "err", err)
			continue
		}
		if root != ph.ID {
			continue // resolved by merge; reads follow the root
		}
		pending, err := s.store.HasProposedMergeFor(ctx, ph.ID)
		if err != nil {
			slog.Warn("judge sweep: pending check failed", "component", "entities", "entity", ph.ID, "err", err)
			continue
		}
		if pending {
			continue
		}
		if err := s.store.PromoteEntity(ctx, ph.ID); err != nil {
			slog.Warn("judge sweep: promote failed", "component", "entities", "entity", ph.ID, "err", err)
			continue
		}
		acted++
	}
	return acted, nil
}

// decidePair applies the escalation ladder to one proposed merge. Returns the outcome
// ("applied" | "rejected" | "unsure" | "" for undecidable-without-judge), who decided
// ("auto" | "llm"), and the evidence to persist under the row's "judge" key.
func (s *JudgeSweeper) decidePair(ctx context.Context, m db.JudgeableMerge) (string, string, map[string]any, error) {
	// Deterministic tier: strong lexical match + compatible kinds.
	if m.Confidence >= judgeAutoMergeMinSim && kindsCompatible(m.FromKind, m.IntoKind) {
		return "applied", "auto", map[string]any{"judge": map[string]any{
			"tier": "deterministic", "sim": m.Confidence,
		}}, nil
	}
	if s.judge == nil {
		return "", "", nil, nil
	}

	aliases, err := s.store.ListEntityAliases(ctx, m.IntoID)
	if err != nil {
		return "", "", nil, fmt.Errorf("aliases: %w", err)
	}
	input := llm.EntityAdjudicationInput{
		Kind:     m.IntoKind,
		A:        m.FromName,
		B:        m.IntoName,
		BAliases: aliases,
	}
	verdict, err := s.judge.JudgeSameEntity(ctx, input)
	if err != nil {
		return "", "", nil, fmt.Errorf("judge: %w", err)
	}
	webUsed := false

	// Web escalation: one search per side, one re-judge, cached upstream (Redis, 7d).
	if verdict.Verdict == "uncertain" && s.searcher != nil {
		snippets, serr := s.webContext(ctx, m.FromName, m.IntoName)
		if serr != nil {
			slog.Warn("judge sweep: web escalation failed", "component", "entities", "merge", m.ID, "err", serr)
		} else if snippets != "" {
			webUsed = true
			input.ContextHint = snippets
			if v2, jerr := s.judge.JudgeSameEntity(ctx, input); jerr == nil {
				verdict = v2
			}
		}
	}

	ev := map[string]any{"judge": map[string]any{
		"tier": "llm", "verdict": verdict.Verdict, "confidence": verdict.Confidence,
		"reason": verdict.Reason, "web": webUsed,
	}}
	switch verdict.Verdict {
	case "same":
		return "applied", "llm", ev, nil
	case "different":
		return "rejected", "llm", ev, nil
	default:
		return "unsure", "llm", ev, nil
	}
}

// webContext searches both names and joins the top snippets into a judge context hint.
func (s *JudgeSweeper) webContext(ctx context.Context, a, b string) (string, error) {
	var parts []string
	for _, q := range []string{a, b} {
		results, err := s.searcher.Search(ctx, q, judgeSearchResults)
		if err != nil {
			return "", err
		}
		for _, r := range results {
			snippet := strings.TrimSpace(r.Content)
			if snippet == "" {
				continue
			}
			if len(snippet) > 300 {
				snippet = snippet[:300]
			}
			parts = append(parts, fmt.Sprintf("[%s] %s", q, snippet))
		}
	}
	return strings.Join(parts, "\n"), nil
}

// kindsCompatible: equal kinds, or either side untyped. Two DIFFERENT typed kinds are
// never deterministically merged — that shape is exactly a sense split.
func kindsCompatible(a, b string) bool {
	if a == b {
		return true
	}
	untyped := func(k string) bool { return k == "" || k == "other" || k == "unknown" }
	return untyped(a) || untyped(b)
}
