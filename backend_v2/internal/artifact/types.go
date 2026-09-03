package artifact

import "encoding/json"

type ArtifactResponse struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"`
	FolderID *string `json:"folderId,omitempty"`
	Title    string  `json:"title"`
	// Content is the canonical body for non-page artifacts (link/voice/file).
	// For pages it is always empty; use Blocks instead.
	Content string `json:"content"`
	// Blocks is the canonical body for pages: a JSON array of BlockNote
	// blocks. Empty (omitted) for non-page artifacts.
	Blocks    json.RawMessage `json:"blocks,omitempty"`
	Summary   *string         `json:"summary"`
	SourceURL *string         `json:"sourceUrl"`
	Metadata  map[string]any  `json:"metadata"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
	Revision  int64           `json:"-"`
	// Seq is the entity's version (the tree-node spine's seq). Populated on the
	// write path (create/update) so the client applies the result under the seq
	// guard; 0/omitted on read paths that don't need it.
	Seq uint64 `json:"seq,string,omitempty"`
}

type BrowserNodeResponse struct {
	ID         string  `json:"id"`
	ParentID   *string `json:"parentId,omitempty"`
	Kind       string  `json:"kind"`
	Title      string  `json:"title"`
	ArtifactID *string `json:"artifactId,omitempty"`
	Position   int64   `json:"position"`
	// Seq is the entity's current version (the staleness gate). A write returns
	// it so the client can apply the result locally under the same seq guard the
	// WS frame uses, making the immediate apply idempotent against the later
	// frame. Wire-encoded as a decimal string (uint64 > JS safe int).
	Seq uint64 `json:"seq,string"`
}

// NodeDeleteResult is the write response for a delete: the tombstoned node's id
// and its bumped version, so the client can apply a seq-guarded soft delete
// locally (the subtree, if any, heals via the WS frame / next pull).
type NodeDeleteResult struct {
	ID  string `json:"id"`
	Seq uint64 `json:"seq,string"`
}

type ArtifactCreateResponse struct {
	Artifact ArtifactResponse    `json:"artifact"`
	Node     BrowserNodeResponse `json:"node"`
}

type BrowserArtifactPayload struct {
	ID        string          `json:"id,omitempty"`
	Type      string          `json:"type"`
	Content   string          `json:"content"`
	Blocks    json.RawMessage `json:"blocks,omitempty"`
	Summary   *string         `json:"summary"`
	SourceURL *string         `json:"sourceUrl"`
	Metadata  map[string]any  `json:"metadata"`
}

type BrowserNodeCreateInput struct {
	ID       string                  `json:"id,omitempty"`
	Kind     string                  `json:"kind"`
	ParentID *string                 `json:"parentId,omitempty"`
	Title    string                  `json:"title"`
	Artifact *BrowserArtifactPayload `json:"artifact,omitempty"`
}

type BrowserNodeCreateResponse struct {
	Node     BrowserNodeResponse `json:"node"`
	Artifact *ArtifactResponse   `json:"artifact,omitempty"`
}

type ArtifactPayload struct {
	ID       string  `json:"id,omitempty"`
	Type     string  `json:"type"`
	FolderID *string `json:"folderId,omitempty"`
	Title    string  `json:"title"`
	// Content is used for non-page artifacts; ignored for pages.
	Content string `json:"content"`
	// Blocks is required for type=page; ignored for other types.
	Blocks    json.RawMessage `json:"blocks,omitempty"`
	Summary   *string         `json:"summary"`
	SourceURL *string         `json:"sourceUrl"`
	Metadata  map[string]any  `json:"metadata"`
}

type ArtifactPatch struct {
	Type      *string          `json:"type"`
	FolderID  *string          `json:"folderId,omitempty"`
	Title     *string          `json:"title"`
	Content   *string          `json:"content"`
	Blocks    *json.RawMessage `json:"blocks,omitempty"`
	Summary   *string          `json:"summary"`
	SourceURL *string          `json:"sourceUrl"`
	Metadata  *map[string]any  `json:"metadata"`
}

type DocumentRecord struct {
	ID         string  `json:"id"`
	Filename   string  `json:"filename"`
	Title      *string `json:"title"`
	Size       *int64  `json:"size,omitempty"`
	PageCount  *int    `json:"page_count"`
	UploadedAt string  `json:"uploaded_at"`
	Ingested   bool    `json:"ingested"`
	Status     string  `json:"status,omitempty"`
}

type AudioUploadRecord struct {
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
}

type NoteRecord struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Label      string         `json:"label"`
	Content    string         `json:"content"`
	Enrichment any            `json:"enrichment"`
	Metadata   map[string]any `json:"metadata"`
	Status     string         `json:"status"`
	CreatedAt  string         `json:"createdAt"`
}

type RelatedRecord struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Label      string  `json:"label"`
	Content    string  `json:"content"`
	SourceURL  *string `json:"sourceUrl"`
	Enrichment any     `json:"enrichment"`
	Similarity float64 `json:"similarity"`
	CreatedAt  string  `json:"createdAt"`
}

type RelatedNotesResponse struct {
	Results []RelatedRecord `json:"results"`
	Message string          `json:"message,omitempty"`
}

type ArtifactListParams struct {
	FolderID *string
}

type PageSearchParams struct {
	Query string
	Limit int
}

// PropertyQuery filters artifacts by a typed property (H1c). Key is required; an empty Value
// matches every artifact carrying the key, which is what a "has a Status" filter wants.
type PropertyQuery struct {
	Key   string
	Value string
	Limit int
}

// PropertyFacet is one property key in use plus the distinct values seen for it — the input a
// filter UI needs to offer real choices instead of free text.
type PropertyFacet struct {
	Key    string   `json:"key"`
	Type   string   `json:"type,omitempty"`
	Values []string `json:"values"`
}

type ArtifactUploadInput struct {
	Type        string
	Filename    string
	ContentType string
	Title       *string
	Summary     *string
	FolderID    *string
}

type ArtifactResource struct {
	Path        string
	ContentType string
}

type FolderNode struct {
	ID       string  `json:"id"`
	ParentID *string `json:"parentId,omitempty"`
	Title    string  `json:"title"`
	Seq      uint64  `json:"seq,string"`
}

type FolderPatch struct {
	Title *string `json:"title"`
}

type BrowserTreeNode struct {
	ID           string            `json:"id"`
	ParentID     *string           `json:"parentId,omitempty"`
	Kind         string            `json:"kind"`
	Title        string            `json:"title"`
	ArtifactID   *string           `json:"artifactId,omitempty"`
	ArtifactType *string           `json:"artifactType,omitempty"`
	UpdatedAt    *string           `json:"updatedAt,omitempty"`
	Children     []BrowserTreeNode `json:"children"`
}

type BrowserTreeFlatNode struct {
	ID           string
	ParentID     *string
	Kind         string
	Title        string
	ArtifactID   *string
	ArtifactType *string
	UpdatedAt    *string
	Position     int64
}

type TreeNodeRecord struct {
	ID         string
	ParentID   *string
	Kind       string
	Title      *string
	ArtifactID *string
	Position   int64
}

type FolderTreeNode struct {
	ID       string           `json:"id"`
	ParentID *string          `json:"parentId,omitempty"`
	Title    string           `json:"title"`
	Children []FolderTreeNode `json:"children"`
}

type BreadcrumbItem struct {
	ID    *string `json:"id,omitempty"`
	Label string  `json:"label"`
}
