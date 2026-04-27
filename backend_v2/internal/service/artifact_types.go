package service

type ArtifactResponse struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	FolderID  *string        `json:"folderId,omitempty"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Summary   *string        `json:"summary"`
	SourceURL *string        `json:"sourceUrl"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt string         `json:"createdAt"`
	UpdatedAt string         `json:"updatedAt"`
}

type ArtifactPayload struct {
	Type      string         `json:"type"`
	FolderID  *string        `json:"folderId,omitempty"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Summary   *string        `json:"summary"`
	SourceURL *string        `json:"sourceUrl"`
	Metadata  map[string]any `json:"metadata"`
}

type ArtifactPatch struct {
	Type      *string         `json:"type"`
	FolderID  *string         `json:"folderId,omitempty"`
	Title     *string         `json:"title"`
	Content   *string         `json:"content"`
	Summary   *string         `json:"summary"`
	SourceURL *string         `json:"sourceUrl"`
	Metadata  *map[string]any `json:"metadata"`
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

type ArtifactUploadInput struct {
	Type     string
	Filename string
	Title    *string
	Summary  *string
	FolderID *string
}

type ArtifactResource struct {
	Path        string
	ContentType string
}

type FolderNode struct {
	ID       string  `json:"id"`
	ParentID *string `json:"parentId,omitempty"`
	Title    string  `json:"title"`
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
