package discourse

import (
	"context"
	"testing"
	"time"

	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/llm"
)

type fakeSource struct {
	bridges []db.Bridge
	members map[string][]db.BridgeMember
	ledger  map[string]*db.EntityDiscourse
	marked  int
	stored  []*db.DiscourseInsight
}

func (f *fakeSource) CandidateBridges(_ context.Context, _, _ int) ([]db.Bridge, error) {
	return f.bridges, nil
}
func (f *fakeSource) BridgeMembers(_ context.Context, id string, _ int) ([]db.BridgeMember, error) {
	return f.members[id], nil
}
func (f *fakeSource) GetDiscourseLedger(_ context.Context, id string) (*db.EntityDiscourse, error) {
	return f.ledger[id], nil
}
func (f *fakeSource) MarkAnalyzed(_ context.Context, _ string, _ int) (int, error) {
	f.marked++
	return 1, nil
}
func (f *fakeSource) StoreDiscourse(_ context.Context, d *db.DiscourseInsight) error {
	f.stored = append(f.stored, d)
	return nil
}

type fakeJudge struct {
	res *llm.DiscourseResult
	err error
}

func (f fakeJudge) JudgeDiscourse(_ context.Context, _ string, _ []llm.DiscourseMember) (*llm.DiscourseResult, error) {
	return f.res, f.err
}

func twoMembers() map[string][]db.BridgeMember {
	return map[string][]db.BridgeMember{
		"e1": {{ID: "r1", Kind: "record", Summary: "a"}, {ID: "r2", Kind: "record", Summary: "b"}},
	}
}

func TestSweep_StoresGroundedDiscourse(t *testing.T) {
	src := &fakeSource{
		bridges: []db.Bridge{{EntityID: "e1", EntityName: "OpenAI", Degree: 2}},
		members: twoMembers(),
		ledger:  map[string]*db.EntityDiscourse{}, // nil → never analyzed → due
	}
	judge := fakeJudge{res: &llm.DiscourseResult{
		Headline: "Sources split on OpenAI", Overall: "mixed", Confidence: 0.7,
		Positions: []llm.DiscoursePosition{
			{MemberID: "r1", Stance: "supportive", Claim: "x"},
			{MemberID: "r2", Stance: "critical", Claim: "y"},
		},
	}}
	n, err := NewService(src, judge, nil).Sweep(context.Background(), 10)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 discourse map, got %d", n)
	}
	if len(src.stored) != 1 || src.stored[0].EntityID != "e1" || len(src.stored[0].MemberIDs) != 2 {
		t.Fatalf("stored = %+v", src.stored)
	}
}

func TestSweep_DropsUngrounded(t *testing.T) {
	src := &fakeSource{
		bridges: []db.Bridge{{EntityID: "e1", EntityName: "X", Degree: 2}},
		members: twoMembers(),
		ledger:  map[string]*db.EntityDiscourse{},
	}
	judge := fakeJudge{res: &llm.DiscourseResult{
		Headline: "h", Overall: "emerging",
		Positions: []llm.DiscoursePosition{{MemberID: "ghost", Stance: "neutral", Claim: "z"}},
	}}
	n, _ := NewService(src, judge, nil).Sweep(context.Background(), 10)
	if n != 0 || len(src.stored) != 0 {
		t.Fatalf("a hallucinated member must ground to nothing and not store: n=%d stored=%d", n, len(src.stored))
	}
}

func TestSweep_SkipsNotDue(t *testing.T) {
	now := time.Now()
	src := &fakeSource{
		bridges: []db.Bridge{{EntityID: "e1", EntityName: "X", Degree: 2}},
		members: twoMembers(),
		ledger:  map[string]*db.EntityDiscourse{"e1": {EntityID: "e1", CountAtLastAnalysis: 2, LastAnalyzedAt: &now, Version: 1}},
	}
	n, _ := NewService(src, fakeJudge{}, nil).Sweep(context.Background(), 10)
	if n != 0 {
		t.Fatalf("a bridge that hasn't grown must be skipped, got n=%d", n)
	}
}
