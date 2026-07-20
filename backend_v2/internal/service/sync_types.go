package service

import "encoding/json"

// Data-layer R1 — the sync wire model (server-authoritative outbox).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// These are plain DTOs (data, not logic): the shape of a sync FRAME as it is
// stored in outbox_events.payload, returned by the pull endpoint, and pushed
// over the realtime websocket. The same shape is decoded by the client engine.

// Op is the per-entity operation in a frame.
type Op string

const (
	OpUpsert Op = "upsert"
	OpDelete Op = "delete"
)

// FrameEntity is one entity's change within a frame.
//
// Seq is the per-entity version (the staleness guard): the client applies this
// entity iff Seq > its stored seq. It is carried on EVERY path (frame + cold
// snapshot). Wire-encoded as a decimal STRING because it is a uint64 and can
// exceed the JS safe-integer range.
//
// Data carries the entity's light fields (what a list/tree view needs) for an
// upsert; it is omitted for a delete (the client soft-deletes by id).
type FrameEntity struct {
	EntityKind string          `json:"entityKind"`
	EntityID   string          `json:"entityId"`
	Seq        uint64          `json:"seq,string"`
	Op         Op              `json:"op"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// Frame is one server transaction's worth of changes — the payload of a
// 'data_event' outbox row. The client applies all entities of a frame
// atomically in one local transaction.
type Frame struct {
	Entities []FrameEntity `json:"entities"`
}

// OutboxAppEvent is a NON-data realtime event carried over the outbox (row type
// 'app_event') so it can cross process boundaries — e.g. shard build-status
// emitted by the MCP build process must reach the API process's websocket.
// Unlike a Frame it is NOT durable synced data: the pull path filters it out
// (clients never store it); only the live drain forwards it, publishing a
// realtime event of type "<ResourceKind>.<Operation>" carrying Payload verbatim.
type OutboxAppEvent struct {
	// Stream routes the event. Empty ⇒ the tenant-scoped workspace stream (back-compat);
	// a broadcast stream (e.g. "market") fans out to all subscribers regardless of tenant.
	Stream       string          `json:"stream,omitempty"`
	ResourceKind string          `json:"resourceKind"`
	ResourceID   string          `json:"resourceId"`
	Operation    string          `json:"operation"`
	Payload      json.RawMessage `json:"payload"`
}

// PullMode tells the client how to apply a pull response.
type PullMode string

const (
	// PullModeDelta — incremental events since the client's cursor; merge them.
	PullModeDelta PullMode = "delta"
	// PullModeSnapshot — the full current state from the canonical tables
	// (cold start / cursor too old); apply as a reconcile-to-exact-set.
	PullModeSnapshot PullMode = "snapshot"
)

// PullResult is the pull endpoint's response. Cursor is the new client cursor
// (the log horizon); wire-encoded as a decimal STRING (uint64 xid).
type PullResult struct {
	Frames []Frame  `json:"frames"`
	Cursor uint64   `json:"cursor,string"`
	Mode   PullMode `json:"mode"`
}
