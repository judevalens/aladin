package blocknote

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// Block represents one top-level BlockNote block addressed by id. The full
// JSON shape is preserved in Raw so we don't lose props/content/children
// while shuttling blocks through Go.
type Block struct {
	ID  string
	Raw json.RawMessage
}

// ErrBlockNotFound is returned by operations that target a block id that
// does not exist in the document.
var ErrBlockNotFound = errors.New("block id not found")

// ParseBlocks splits a BlockNote document (a JSON array of blocks) into
// individual Block values. Blocks without an `id` field are accepted —
// their ID will be empty. The caller can assign IDs before splicing them
// in via WithID.
func ParseBlocks(doc json.RawMessage) ([]Block, error) {
	if len(bytes.TrimSpace(doc)) == 0 {
		return nil, nil
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(doc, &raws); err != nil {
		return nil, fmt.Errorf("blocknote: parse blocks: %w", err)
	}
	blocks := make([]Block, len(raws))
	for i, r := range raws {
		id, err := blockID(r)
		if err != nil {
			return nil, fmt.Errorf("blocknote: parse block %d: %w", i, err)
		}
		blocks[i] = Block{ID: id, Raw: r}
	}
	return blocks, nil
}

// SerializeBlocks marshals a slice of Block back into a JSON array.
func SerializeBlocks(blocks []Block) (json.RawMessage, error) {
	if len(blocks) == 0 {
		return json.RawMessage(`[]`), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, b := range blocks {
		if i > 0 {
			buf.WriteByte(',')
		}
		if len(b.Raw) == 0 {
			return nil, fmt.Errorf("blocknote: block %d has empty raw payload", i)
		}
		buf.Write(b.Raw)
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// FindByID returns the index of the block with id, or -1.
func FindByID(blocks []Block, id string) int {
	for i, b := range blocks {
		if b.ID == id {
			return i
		}
	}
	return -1
}

// ReplaceByID swaps the block with `id` for `replacement` (which may be
// multiple blocks). The first block in `replacement` inherits `id` to keep
// downstream references stable; subsequent blocks keep their own ids.
// Returns the new slice, the number of replacement blocks inserted, and
// ErrBlockNotFound if `id` does not exist.
func ReplaceByID(blocks []Block, id string, replacement []Block) ([]Block, int, error) {
	idx := FindByID(blocks, id)
	if idx < 0 {
		return blocks, 0, ErrBlockNotFound
	}
	if len(replacement) == 0 {
		// Treat empty replacement as deletion semantics for callers that
		// pass [] markdown; not what update_block typically does, but a
		// defensible default.
		out := make([]Block, 0, len(blocks)-1)
		out = append(out, blocks[:idx]...)
		out = append(out, blocks[idx+1:]...)
		return out, 0, nil
	}
	rewritten := make([]Block, len(replacement))
	copy(rewritten, replacement)
	first, err := WithID(rewritten[0], id)
	if err != nil {
		return blocks, 0, err
	}
	rewritten[0] = first

	out := make([]Block, 0, len(blocks)-1+len(rewritten))
	out = append(out, blocks[:idx]...)
	out = append(out, rewritten...)
	out = append(out, blocks[idx+1:]...)
	return out, len(rewritten), nil
}

// InsertAfter inserts `newBlocks` immediately after the block with `id`.
func InsertAfter(blocks []Block, id string, newBlocks []Block) ([]Block, error) {
	idx := FindByID(blocks, id)
	if idx < 0 {
		return blocks, ErrBlockNotFound
	}
	return spliceAt(blocks, idx+1, newBlocks), nil
}

// InsertBefore inserts `newBlocks` immediately before the block with `id`.
func InsertBefore(blocks []Block, id string, newBlocks []Block) ([]Block, error) {
	idx := FindByID(blocks, id)
	if idx < 0 {
		return blocks, ErrBlockNotFound
	}
	return spliceAt(blocks, idx, newBlocks), nil
}

// InsertAtStart prepends new blocks.
func InsertAtStart(blocks []Block, newBlocks []Block) []Block {
	return spliceAt(blocks, 0, newBlocks)
}

// InsertAtEnd appends new blocks.
func InsertAtEnd(blocks []Block, newBlocks []Block) []Block {
	return spliceAt(blocks, len(blocks), newBlocks)
}

// DeleteByID removes the block with id.
func DeleteByID(blocks []Block, id string) ([]Block, error) {
	idx := FindByID(blocks, id)
	if idx < 0 {
		return blocks, ErrBlockNotFound
	}
	out := make([]Block, 0, len(blocks)-1)
	out = append(out, blocks[:idx]...)
	out = append(out, blocks[idx+1:]...)
	return out, nil
}

// WithID returns a copy of `b` whose `id` field is set to the given value
// (preserving everything else in the block's JSON).
func WithID(b Block, id string) (Block, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b.Raw, &obj); err != nil {
		return Block{}, fmt.Errorf("blocknote: WithID parse: %w", err)
	}
	encoded, err := json.Marshal(id)
	if err != nil {
		return Block{}, err
	}
	obj["id"] = encoded
	raw, err := json.Marshal(obj)
	if err != nil {
		return Block{}, err
	}
	return Block{ID: id, Raw: raw}, nil
}

// CollectIDs returns the ordered list of block ids.
func CollectIDs(blocks []Block) []string {
	out := make([]string, len(blocks))
	for i, b := range blocks {
		out[i] = b.ID
	}
	return out
}

func spliceAt(blocks []Block, at int, newBlocks []Block) []Block {
	if at < 0 {
		at = 0
	}
	if at > len(blocks) {
		at = len(blocks)
	}
	out := make([]Block, 0, len(blocks)+len(newBlocks))
	out = append(out, blocks[:at]...)
	out = append(out, newBlocks...)
	out = append(out, blocks[at:]...)
	return out
}

func blockID(raw json.RawMessage) (string, error) {
	var probe struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return "", err
	}
	return probe.ID, nil
}
