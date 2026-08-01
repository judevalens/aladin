package blocknote

import (
	"encoding/json"
	"testing"
)

func TestExtractInlineRefs(t *testing.T) {
	doc := json.RawMessage(`[
		{"id":"b1","type":"paragraph","content":[
			{"type":"text","text":"per "},
			{"type":"entityMention","props":{"entityId":"e-1","label":"OpenAI","kind":"org"}},
			{"type":"artifactRef","props":{"kind":"shard","targetId":"art_9","label":"Dashboard"}}
		],"children":[
			{"id":"b2","type":"paragraph","content":[
				{"type":"artifactRef","props":{"kind":"page","targetId":"art_7","label":"Roadmap"}}
			]}
		]},
		{"id":"b3","type":"paragraph","content":[
			{"type":"entityMention","props":{"entityId":"e-1","label":"OpenAI"}},
			{"type":"artifactRef","props":{"kind":"bogus","targetId":"x","label":"nope"}},
			{"type":"artifactRef","props":{"kind":"claim","targetId":"c-9","label":"retired kind"}},
			{"type":"artifactRef","props":{"kind":"shard","targetId":"","label":"empty"}}
		]},
		{"id":"b4","type":"table","content":{"type":"tableContent","rows":[]}}
	]`)

	mentions, refs, err := ExtractInlineRefs(doc)
	if err != nil {
		t.Fatalf("ExtractInlineRefs: %v", err)
	}

	// e-1 mentioned in b1 and b3 → 2 distinct (entity, block) rows.
	if len(mentions) != 2 {
		t.Fatalf("mentions = %+v (want 2)", mentions)
	}
	if mentions[0].EntityID != "e-1" || mentions[0].BlockID != "b1" || mentions[0].Surface != "OpenAI" {
		t.Fatalf("mention[0] = %+v", mentions[0])
	}

	// shard (b1), page (b2, nested child) — bogus-kind and empty-target dropped. A retired
	// kind (the removed claim layer) is dropped by the same whitelist as "bogus".
	if len(refs) != 2 {
		t.Fatalf("refs = %+v (want 2: shard + nested page)", refs)
	}
	byKind := map[string]ArtifactReference{}
	for _, r := range refs {
		byKind[r.Kind] = r
	}
	if byKind["shard"].TargetID != "art_9" || byKind["shard"].BlockID != "b1" {
		t.Fatalf("shard ref = %+v", byKind["shard"])
	}
	if byKind["page"].TargetID != "art_7" || byKind["page"].BlockID != "b2" {
		t.Fatalf("page ref (nested child) = %+v", byKind["page"])
	}
	if _, bad := byKind["bogus"]; bad {
		t.Fatalf("bogus kind should be dropped: %+v", refs)
	}
	if _, retired := byKind["claim"]; retired {
		t.Fatalf("the retired claim kind should be dropped: %+v", refs)
	}
}

func TestExtractInlineRefs_EmptyAndMalformed(t *testing.T) {
	for _, doc := range []json.RawMessage{
		json.RawMessage(``),
		json.RawMessage(`[]`),
		json.RawMessage(`{"not":"an array"}`),
	} {
		m, r, err := ExtractInlineRefs(doc)
		if err != nil {
			t.Fatalf("doc %s: %v", doc, err)
		}
		if len(m) != 0 || len(r) != 0 {
			t.Fatalf("doc %s: expected empty, got m=%v r=%v", doc, m, r)
		}
	}
}
