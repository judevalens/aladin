package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// The entity registry — the workspace plane's read surface for shards
// (SHARD_MODEL §6). A shard declares the entities it depends on as `refs` in
// anchors.json; the bridge resolves each ref to a kind + a service that can
// answer "does it exist" and "give me its view". The registry itself has NO
// notion of shards or grants: it just maps a ref to a reader. ShardBridgeService
// enforces the grant on top.
//
// Ref encoding follows the ids the app actually mints (service/helpers.go
// newID): prefixed kinds are self-describing ("artifact-…", "record-…",
// "research-…"); kinds whose ids are bare uuids take an explicit qualifier
// ("watchlist:<uuid>") — SHARD_MODEL §6's "same information, two encodings".

// NodeView is what a shard sees of one workspace entity: enough to render, never
// the full record. Data is kind-shaped JSON (documented per adapter); Seq is the
// entity's sync version, used by the host to drop out-of-order pushes.
type NodeView struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Data      json.RawMessage `json:"data,omitempty"`
	Seq       uint64          `json:"seq,string,omitempty"`
	Truncated bool            `json:"truncated,omitempty"`
}

// NodeViewMaxBytes caps any single free-text field carried in a NodeView. The
// bridge is for PROJECTION — a shard that needs a whole document should render
// a link, not inline the body.
const NodeViewMaxBytes = 16 << 10

// TruncateForView clips s to the cap, reporting whether it clipped.
func TruncateForView(s string) (string, bool) {
	if len(s) <= NodeViewMaxBytes {
		return s, false
	}
	return s[:NodeViewMaxBytes], true
}

// EntityService reads one kind of workspace entity for the bridge. Implementations
// wrap the kind's existing SERVICE (not its repo) so ownership/scope rules stay in
// one place.
type EntityService interface {
	// Kind is the registry key ("artifact", "record", …).
	Kind() string
	// Exists reports whether the id resolves for the ctx principal — the
	// publish-time refs check, without paying for a full view.
	Exists(ctx context.Context, id string) (bool, error)
	// NodeView renders the entity for a shard.
	NodeView(ctx context.Context, id string) (NodeView, error)
	// Observable reports whether this kind emits sync frames. A ref that isn't
	// observable can be rendered but never goes live — the publish gate warns,
	// and a `live` binding on it hard-fails.
	Observable() bool
}

// EntityRegistry resolves refs to their EntityService.
type EntityRegistry struct {
	byPrefix map[string]EntityService // "artifact-" -> svc
	byKind   map[string]EntityService // "watchlist" -> svc (explicit qualifier)
}

// entityPrefixes maps a kind to the id prefix it mints, where it mints one.
// Kinds absent here are addressed with the "kind:<id>" qualifier form.
var entityPrefixes = map[string]string{
	"artifact": "artifact-",
	"record":   "record-",
	"research": "research-",
}

func NewEntityRegistry(services ...EntityService) *EntityRegistry {
	r := &EntityRegistry{byPrefix: map[string]EntityService{}, byKind: map[string]EntityService{}}
	for _, svc := range services {
		if svc == nil {
			continue
		}
		r.byKind[svc.Kind()] = svc
		if prefix, ok := entityPrefixes[svc.Kind()]; ok {
			r.byPrefix[prefix] = svc
		}
	}
	return r
}

// Kinds lists the registered kinds (for hello/capabilities + diagnostics).
func (r *EntityRegistry) Kinds() []string {
	out := make([]string, 0, len(r.byKind))
	for k := range r.byKind {
		out = append(out, k)
	}
	return out
}

// Resolve maps a ref to its kind, canonical id, and reader. The canonical id is
// the id the underlying service expects: prefixed kinds keep the whole ref (the
// prefix is part of the id), qualified kinds drop the "kind:" qualifier.
func (r *EntityRegistry) Resolve(ref string) (kind, id string, svc EntityService, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", nil, BadRequest("empty ref")
	}
	// Longest-prefix match first: prefixed ids are self-describing.
	best := ""
	for prefix := range r.byPrefix {
		if strings.HasPrefix(ref, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best != "" {
		s := r.byPrefix[best]
		return s.Kind(), ref, s, nil
	}
	// Otherwise an explicit "kind:<id>" qualifier.
	if k, rest, ok := strings.Cut(ref, ":"); ok && rest != "" {
		if s, known := r.byKind[k]; known {
			return k, rest, s, nil
		}
		return "", "", nil, BadRequest(fmt.Sprintf("unknown entity kind %q in ref %q", k, ref))
	}
	return "", "", nil, BadRequest(fmt.Sprintf("unrecognized ref %q: use a prefixed id (artifact-…) or kind:id (watchlist:…)", ref))
}
