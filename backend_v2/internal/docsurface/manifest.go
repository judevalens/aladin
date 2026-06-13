package docsurface

import (
	"encoding/json"
	"fmt"
)

// The anchor manifest (anchors.json) is the canonical identity layer: it declares
// a shard's addressable regions and joins each to the knowledge graph. See
// design/SHARD_MODEL.md. v1 validates STRUCTURE here (schema, hard at build);
// the live checks (anchor resolves on its route, refs exist, conservation
// coverage) ride the preview harness and are added separately.

// ManifestFileName is the page-relative path of the manifest.
const ManifestFileName = "anchors.json"

// Manifest is the parsed anchors.json. Binding is kept opaque (json.RawMessage)
// at the schema layer — it is provenance the agent authors (prose, or
// {evaluation, derivation}); its shape is deliberately not constrained here.
type Manifest struct {
	Version int              `json:"version"`
	Intent  string           `json:"intent,omitempty"`
	Anchors []ManifestAnchor `json:"anchors"`
}

type ManifestAnchor struct {
	ID      string          `json:"id"`
	Kind    string          `json:"kind,omitempty"`
	Route   string          `json:"route"`
	Source  string          `json:"source,omitempty"`
	Binding json.RawMessage `json:"binding,omitempty"`
	Refs    []string        `json:"refs,omitempty"`
	Meaning string          `json:"meaning"`
}

// knownAnchorKinds is the advisory region taxonomy (mirrors the kit). `kind` is
// an OPEN, kit-owned string — NOT enforced here — so the kit can introduce new
// kinds without a manifest-schema change. Kept for documentation / a future
// soft warning.
var knownAnchorKinds = map[string]bool{
	"collection": true,
	"metric":     true,
	"chart":      true,
	"narrative":  true,
	"control":    true,
}

var _ = knownAnchorKinds // advisory only; see above

// ParseManifest decodes anchors.json. A decode error is returned as a single
// validation message via ValidateManifestBytes; callers usually use that.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// ValidateManifestBytes parses + validates raw anchors.json, returning a list of
// human-readable problems (empty = valid). A parse failure is reported as one
// problem rather than a Go error so it composes into the build log.
func ValidateManifestBytes(data []byte) []string {
	m, err := ParseManifest(data)
	if err != nil {
		return []string{"anchors.json is not valid JSON: " + err.Error()}
	}
	return ValidateManifest(m)
}

// ValidateManifest checks the manifest's structure: version, required per-anchor
// fields, the optional kind enum, and id-uniqueness per route.
func ValidateManifest(m Manifest) []string {
	var problems []string
	if m.Version != 1 {
		problems = append(problems, fmt.Sprintf("version must be 1 (got %d)", m.Version))
	}
	seen := map[string]int{} // route\x00id -> first index, for per-route uniqueness
	for i, a := range m.Anchors {
		where := fmt.Sprintf("anchors[%d]", i)
		if a.ID != "" {
			where = fmt.Sprintf("anchor %q", a.ID)
		}
		if a.ID == "" {
			problems = append(problems, where+": id is required")
		}
		if a.Route == "" {
			problems = append(problems, where+": route is required")
		}
		if a.Meaning == "" {
			problems = append(problems, where+": meaning is required")
		}
		// kind is intentionally NOT validated — it's an open, kit-owned string.
		if a.ID != "" && a.Route != "" {
			key := a.Route + "\x00" + a.ID
			if _, dup := seen[key]; dup {
				problems = append(problems, fmt.Sprintf("duplicate anchor id %q on route %q", a.ID, a.Route))
			}
			seen[key] = i
		}
	}
	return problems
}
