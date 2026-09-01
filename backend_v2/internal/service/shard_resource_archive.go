package service

import (
	"aladin/backend_v2/internal/shardv2"
	"context"
	"io"
)

type ResourceArchiveManifest struct {
	SHA256   string `json:"sha256"`
	Records  int    `json:"records"`
	Receipts int    `json:"receipts"`
}

// Recovery is deliberately internal, not an authored bridge or MCP operation.
// Restore requires an empty namespace and the exact active contract/generation;
// it never overwrites a namespace that has accepted writes after an export.
type ResourceArchiveStore interface {
	ExportResourceData(context.Context, string, BuildChannel, io.Writer) (ResourceArchiveManifest, error)
	RestoreResourceData(context.Context, string, BuildChannel, io.Reader, shardv2.Registry) (ResourceArchiveManifest, error)
}
