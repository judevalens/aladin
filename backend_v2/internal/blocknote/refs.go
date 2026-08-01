package blocknote

import (
	"bytes"
	"encoding/json"
)

// EntityMention is one @entity occurrence extracted from a page's blocks.
type EntityMention struct {
	EntityID string
	BlockID  string
	Surface  string
}

// ArtifactReference is one `#` reference (page | shard) extracted from a page's
// blocks.
type ArtifactReference struct {
	Kind     string
	TargetID string
	BlockID  string
	Surface  string
}

type refWalkBlock struct {
	ID       string          `json:"id"`
	Content  json.RawMessage `json:"content"`
	Children json.RawMessage `json:"children"`
}

type refWalkInline struct {
	Type  string `json:"type"`
	Props struct {
		EntityID string `json:"entityId"`
		Label    string `json:"label"`
		Kind     string `json:"kind"`
		TargetID string `json:"targetId"`
	} `json:"props"`
}

// ExtractInlineRefs walks a BlockNote document (a JSON array of blocks, with nested
// children) and pulls out every @entity mention and `#` artifact reference, deduped by
// (target, block). It mirrors the frontend extractors (extractEntityMentions /
// extractArtifactRefs) so a page written via MCP reconciles the same rows the editor would.
// Non-array block content (e.g. tables) is skipped rather than erroring.
func ExtractInlineRefs(doc json.RawMessage) ([]EntityMention, []ArtifactReference, error) {
	mentions := []EntityMention{}
	refs := []ArtifactReference{}
	seenM := map[string]bool{}
	seenR := map[string]bool{}

	var walk func(raw json.RawMessage)
	walk = func(raw json.RawMessage) {
		if len(bytes.TrimSpace(raw)) == 0 {
			return
		}
		var blocks []refWalkBlock
		if err := json.Unmarshal(raw, &blocks); err != nil {
			return // not a block array → nothing to walk
		}
		for _, b := range blocks {
			var inline []refWalkInline
			if len(bytes.TrimSpace(b.Content)) > 0 {
				_ = json.Unmarshal(b.Content, &inline) // ignore non-array content
			}
			for _, ic := range inline {
				switch ic.Type {
				case "entityMention":
					if ic.Props.EntityID == "" {
						continue
					}
					key := ic.Props.EntityID + "\x00" + b.ID
					if seenM[key] {
						continue
					}
					seenM[key] = true
					mentions = append(mentions, EntityMention{
						EntityID: ic.Props.EntityID,
						BlockID:  b.ID,
						Surface:  ic.Props.Label,
					})
				case "artifactRef":
					k := ic.Props.Kind
					if ic.Props.TargetID == "" || (k != "page" && k != "shard") {
						continue
					}
					key := k + "\x00" + ic.Props.TargetID + "\x00" + b.ID
					if seenR[key] {
						continue
					}
					seenR[key] = true
					refs = append(refs, ArtifactReference{
						Kind:     k,
						TargetID: ic.Props.TargetID,
						BlockID:  b.ID,
						Surface:  ic.Props.Label,
					})
				}
			}
			walk(b.Children)
		}
	}

	walk(doc)
	return mentions, refs, nil
}
