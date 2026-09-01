package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aladin/backend_v2/internal/app"
	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/service"
)

type releaseMetadataStub struct {
	service.ShardReleaseService
	release service.ShardRelease
	err     error
}

func (s releaseMetadataStub) Active(context.Context, string, service.BuildChannel) (service.ShardRelease, error) {
	return s.release, s.err
}

func TestShardReleaseMetadataSeparatesUnpublishedFromUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name      string
		legacy    bool
		protected bool
		disabled  bool
	}{
		{name: "no published build"},
		{name: "legacy published build", legacy: true},
		{name: "protected publication", protected: true},
		{name: "disabled protected publication is not a legacy fallback", legacy: true, disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := docsurface.NewStore(t.TempDir())
			ctx := service.WithPrincipal(context.Background(), service.Principal{UserID: "user-1", ActorType: service.ActorTypeUserSession})
			if _, err := store.EnsurePageDir(ctx, "artifact-shard"); err != nil {
				t.Fatal(err)
			}
			// A draft alone must never be advertised as published content.
			if err := store.WriteFile(ctx, "artifact-shard", "dist/draft/bundle.js", []byte("draft")); err != nil {
				t.Fatal(err)
			}
			if tc.legacy {
				if err := store.WriteFile(ctx, "artifact-shard", "dist/bundle.js", []byte("published")); err != nil {
					t.Fatal(err)
				}
			}
			releases := releaseMetadataStub{err: service.ErrNotFound}
			if tc.protected {
				releases.err = nil
				releases.release = service.ShardRelease{ResourceRelease: service.ResourceRelease{BuildID: "active-build", Hash: "active-hash"}}
			}
			if tc.disabled {
				releases.err = service.ResourceFailure("unsupported-capability", "disabled")
			}
			server := NewWithDependencies(":0", app.StaticDependencies{AuthSvc: &fakeAuthService{}, ArtifactsSvc: shardArtifactService{}, DocSurfaceStoreSvc: store, ShardReleaseSvc: releases})
			req := httptest.NewRequest(http.MethodGet, "/api/shards/artifact-shard/release?channel=published", nil)
			req.Header.Set("Authorization", "Bearer desktop-valid")
			rec := httptest.NewRecorder()
			server.httpServer.Handler.ServeHTTP(rec, req)
			if tc.disabled {
				if rec.Code != http.StatusServiceUnavailable {
					t.Fatalf("disabled runtime: %d %s", rec.Code, rec.Body.String())
				}
				return
			}
			var result map[string]any
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil {
				t.Fatalf("metadata: %d %s", rec.Code, rec.Body.String())
			}
			if tc.protected {
				if result["protocol"] != "bridge/2" || result["buildId"] != "active-build" || result["contractHash"] != "active-hash" {
					t.Fatalf("wrong protected release: %+v", result)
				}
			} else if result["protocol"] != "bridge/1" || result["available"] != tc.legacy {
				t.Fatalf("wrong availability: %+v", result)
			}
		})
	}
}
