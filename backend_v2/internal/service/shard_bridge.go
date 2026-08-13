package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// The workspace plane's grant layer (SHARD_MODEL §6). A shard may read ONLY the
// entities it declared in anchors.json `refs` — the manifest is the capability
// list, and this service is where that is enforced, server-side, on every call.
// The host cannot be the only check: a hostile shard can post any ids it likes.
//
// The grant is NOT transitive: there is no nodes.related / neighborhood read. A
// shard that wants another entity must declare it, which keeps the manifest an
// honest description of what the shard depends on.

// shardManifestFile is the page-relative manifest path. Duplicated from
// docsurface.ManifestFileName because docsurface imports this package, not the
// other way round; the docsurface test suite asserts they agree.
const shardManifestFile = "anchors.json"

// RefsReport is the publish-time audit of a manifest's refs.
type RefsReport struct {
	// OK is true when nothing is missing and no unknown kinds appear.
	OK bool `json:"ok"`
	// Missing refs resolve to a known kind but no such entity (or not visible).
	Missing []string `json:"missing,omitempty"`
	// UnknownKind refs don't resolve to any registered kind.
	UnknownKind []string `json:"unknown_kind,omitempty"`
	// Unobservable refs exist but their kind emits no sync frames — a region
	// bound to them can render but never update.
	Unobservable []string `json:"unobservable,omitempty"`
	// Total is how many distinct refs the manifest declared.
	Total int `json:"total"`
}

type ShardBridgeService interface {
	// GetNodes returns the views for ids, enforcing the manifest grant. Any id
	// outside the grant fails the WHOLE call (a shard asking for something it
	// didn't declare is a bug to surface, not a subset to silently serve).
	// Ids that pass the grant but don't resolve come back in missing.
	GetNodes(ctx context.Context, shardID string, ids []string) (nodes []NodeView, missing []string, err error)
	// CheckRefs audits every ref a shard declares — the publish gate's input.
	CheckRefs(ctx context.Context, shardID string) (RefsReport, error)
}

type shardBridgeService struct {
	artifacts ArtifactService
	store     DocSurfaceStore
	registry  *EntityRegistry
}

func NewShardBridgeService(artifacts ArtifactService, store DocSurfaceStore, registry *EntityRegistry) ShardBridgeService {
	return &shardBridgeService{artifacts: artifacts, store: store, registry: registry}
}

// grantFor reads the shard's declared refs. Read PER CALL: anchors.json is a
// small local file and a draft build can rewrite it, so caching would open a
// stale-grant window for no measurable gain.
func (s *shardBridgeService) grantFor(ctx context.Context, shardID string) (map[string]bool, error) {
	rec, err := s.artifacts.Get(ctx, shardID)
	if err != nil {
		return nil, err
	}
	if rec.Type != "app" {
		return nil, ErrNotFound
	}
	data, err := s.store.ReadFile(ctx, shardID, shardManifestFile)
	if err != nil {
		// No manifest = no declared refs = an empty grant, not an error: the
		// shard simply can't read workspace data until it declares some.
		return map[string]bool{}, nil
	}
	var m struct {
		Anchors []struct {
			Refs []string `json:"refs"`
		} `json:"anchors"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, BadRequest("anchors.json is not valid JSON: " + err.Error())
	}
	grant := map[string]bool{}
	for _, a := range m.Anchors {
		for _, ref := range a.Refs {
			if ref = strings.TrimSpace(ref); ref != "" {
				grant[ref] = true
			}
		}
	}
	return grant, nil
}

func (s *shardBridgeService) GetNodes(ctx context.Context, shardID string, ids []string) ([]NodeView, []string, error) {
	grant, err := s.grantFor(ctx, shardID)
	if err != nil {
		return nil, nil, err
	}
	var denied []string
	for _, id := range ids {
		if !grant[strings.TrimSpace(id)] {
			denied = append(denied, id)
		}
	}
	if len(denied) > 0 {
		sort.Strings(denied)
		return nil, nil, fmt.Errorf("%w: not declared in anchors.json refs: %s",
			ErrForbidden, strings.Join(denied, ", "))
	}

	nodes := make([]NodeView, 0, len(ids))
	var missing []string
	for _, ref := range ids {
		_, canonicalID, svc, rerr := s.registry.Resolve(ref)
		if rerr != nil {
			missing = append(missing, ref)
			continue
		}
		view, verr := svc.NodeView(ctx, canonicalID)
		if verr != nil {
			// Not-found (or not visible) is data, not an error: the shard
			// renders an empty region rather than failing the whole call.
			missing = append(missing, ref)
			continue
		}
		// Answer under the REF the shard asked with, so its keys line up.
		view.ID = ref
		nodes = append(nodes, view)
	}
	return nodes, missing, nil
}

func (s *shardBridgeService) CheckRefs(ctx context.Context, shardID string) (RefsReport, error) {
	grant, err := s.grantFor(ctx, shardID)
	if err != nil {
		return RefsReport{}, err
	}
	report := RefsReport{Total: len(grant)}
	refs := make([]string, 0, len(grant))
	for ref := range grant {
		refs = append(refs, ref)
	}
	sort.Strings(refs)

	for _, ref := range refs {
		_, canonicalID, svc, rerr := s.registry.Resolve(ref)
		if rerr != nil {
			report.UnknownKind = append(report.UnknownKind, ref)
			continue
		}
		ok, eerr := svc.Exists(ctx, canonicalID)
		if eerr != nil {
			return RefsReport{}, eerr
		}
		if !ok {
			report.Missing = append(report.Missing, ref)
			continue
		}
		if !svc.Observable() {
			report.Unobservable = append(report.Unobservable, ref)
		}
	}
	report.OK = len(report.Missing) == 0 && len(report.UnknownKind) == 0
	return report, nil
}
