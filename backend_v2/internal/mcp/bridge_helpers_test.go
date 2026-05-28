package mcpserver

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"aladin/backend_v2/internal/blocknote"
	"aladin/backend_v2/internal/service"
)

func TestWithFirstBlockID(t *testing.T) {
	t.Parallel()

	t.Run("overrides first id, leaves the rest", func(t *testing.T) {
		out := withFirstBlockID(
			json.RawMessage(`[{"id":"old","type":"paragraph"},{"id":"keep","type":"paragraph"}]`),
			"new",
		)
		var arr []map[string]any
		if err := json.Unmarshal(out, &arr); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if arr[0]["id"] != "new" {
			t.Fatalf("first id = %v, want new", arr[0]["id"])
		}
		if arr[1]["id"] != "keep" {
			t.Fatalf("second id = %v, want keep (untouched)", arr[1]["id"])
		}
	})

	t.Run("adds id when first block has none", func(t *testing.T) {
		out := withFirstBlockID(json.RawMessage(`[{"type":"paragraph"}]`), "added")
		if !strings.Contains(string(out), `"added"`) {
			t.Fatalf("out = %s, want id added", out)
		}
	})

	t.Run("empty array unchanged", func(t *testing.T) {
		out := withFirstBlockID(json.RawMessage(`[]`), "x")
		if strings.TrimSpace(string(out)) != "[]" {
			t.Fatalf("out = %s, want []", out)
		}
	})

	t.Run("malformed json unchanged", func(t *testing.T) {
		out := withFirstBlockID(json.RawMessage(`not json`), "x")
		if string(out) != "not json" {
			t.Fatalf("out = %s, want unchanged", out)
		}
	})
}

func TestApplyInsertPosition(t *testing.T) {
	t.Parallel()

	t.Run("after_id → anchor + after", func(t *testing.T) {
		var op blocknote.BridgeOp
		if err := applyInsertPosition(&op, insertBlocksPositionDTO{AfterID: stringPtr("b1")}); err != nil {
			t.Fatal(err)
		}
		if op.BlockID != "b1" || op.Placement != "after" {
			t.Fatalf("op = %+v, want b1/after", op)
		}
	})

	t.Run("before_id → anchor + before", func(t *testing.T) {
		var op blocknote.BridgeOp
		if err := applyInsertPosition(&op, insertBlocksPositionDTO{BeforeID: stringPtr("b2")}); err != nil {
			t.Fatal(err)
		}
		if op.BlockID != "b2" || op.Placement != "before" {
			t.Fatalf("op = %+v, want b2/before", op)
		}
	})

	t.Run("at start → index 0", func(t *testing.T) {
		var op blocknote.BridgeOp
		if err := applyInsertPosition(&op, insertBlocksPositionDTO{At: stringPtr("start")}); err != nil {
			t.Fatal(err)
		}
		if op.Position == nil || *op.Position != 0 {
			t.Fatalf("position = %v, want 0", op.Position)
		}
	})

	t.Run("at end → append (no anchor/index)", func(t *testing.T) {
		var op blocknote.BridgeOp
		if err := applyInsertPosition(&op, insertBlocksPositionDTO{At: stringPtr("end")}); err != nil {
			t.Fatal(err)
		}
		if op.BlockID != "" || op.Position != nil {
			t.Fatalf("op = %+v, want append", op)
		}
	})

	t.Run("unset → append", func(t *testing.T) {
		var op blocknote.BridgeOp
		if err := applyInsertPosition(&op, insertBlocksPositionDTO{}); err != nil {
			t.Fatal(err)
		}
		if op.BlockID != "" || op.Position != nil {
			t.Fatalf("op = %+v, want append", op)
		}
	})

	t.Run("ambiguous after+before rejected", func(t *testing.T) {
		var op blocknote.BridgeOp
		err := applyInsertPosition(&op, insertBlocksPositionDTO{
			AfterID:  stringPtr("a"),
			BeforeID: stringPtr("b"),
		})
		var bad service.BadRequest
		if !errors.As(err, &bad) {
			t.Fatalf("err = %v, want BadRequest", err)
		}
	})

	t.Run("unknown at rejected", func(t *testing.T) {
		var op blocknote.BridgeOp
		err := applyInsertPosition(&op, insertBlocksPositionDTO{At: stringPtr("middle")})
		var bad service.BadRequest
		if !errors.As(err, &bad) {
			t.Fatalf("err = %v, want BadRequest", err)
		}
	})
}
