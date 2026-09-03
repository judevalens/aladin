package instrument

import (
	"context"
	"testing"
)

type fakeRepository struct {
	ids     map[string]string
	upserts int
}

func (*fakeRepository) SearchInstruments(context.Context, string, int) ([]InstrumentHit, error) {
	return nil, nil
}
func (f *fakeRepository) UpsertInstruments(_ context.Context, rows []InstrumentUpsert) (int, error) {
	for _, row := range rows {
		f.ids[row.Symbol] = "id-" + row.Symbol
	}
	f.upserts++
	return len(rows), nil
}
func (f *fakeRepository) ResolveInstrumentID(_ context.Context, symbol string) (string, bool, error) {
	id, ok := f.ids[symbol]
	return id, ok, nil
}

type fakeLookup struct{ calls int }

func (f *fakeLookup) FetchInstrument(_ context.Context, symbol string) (InstrumentUpsert, bool, error) {
	f.calls++
	return InstrumentUpsert{Symbol: symbol, IsActive: true}, true, nil
}

func TestResolveReadThroughOnMiss(t *testing.T) {
	repository := &fakeRepository{ids: map[string]string{}}
	lookup := &fakeLookup{}
	service := NewInstrumentService(repository).WithAssetLookup(lookup)

	id, ok, err := service.ResolveInstrumentID(context.Background(), "ZZZ")
	if err != nil || !ok || id != "id-ZZZ" || lookup.calls != 1 || repository.upserts != 1 {
		t.Fatalf("read-through: id=%q ok=%v calls=%d upserts=%d err=%v", id, ok, lookup.calls, repository.upserts, err)
	}
	if _, _, err := service.ResolveInstrumentID(context.Background(), "ZZZ"); err != nil || lookup.calls != 1 {
		t.Fatalf("cached: calls=%d err=%v (want 1)", lookup.calls, err)
	}
}
