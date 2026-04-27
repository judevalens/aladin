package repo

import (
	"encoding/json"
	"strings"
	"time"

	coreservice "aladin/backend_v2/internal/service"
)

type scanner interface{ Scan(dest ...any) error }

func scanRecordRow(row scanner, withChildCount bool) (coreservice.RecordResponse, error) {
	var rec coreservice.RecordResponse
	var sourceURL *string
	var parentID *string
	var enrichment []byte
	var metadata []byte
	var createdAt time.Time
	rec.Metadata = map[string]any{}
	if withChildCount {
		if err := row.Scan(&rec.ID, &rec.Type, &rec.Label, &rec.Content, &sourceURL, &parentID, &enrichment, &metadata, &createdAt, &rec.ChildCount); err != nil {
			return rec, err
		}
	} else {
		if err := row.Scan(&rec.ID, &rec.Type, &rec.Label, &rec.Content, &sourceURL, &parentID, &enrichment, &metadata, &createdAt); err != nil {
			return rec, err
		}
	}
	if len(enrichment) > 0 {
		_ = json.Unmarshal(enrichment, &rec.Enrichment)
	}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &rec.Metadata)
	}
	rec.SourceURL = sourceURL
	rec.ParentID = parentID
	rec.CreatedAt = createdAt.Format(time.RFC3339)
	return rec, nil
}

func nilIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
