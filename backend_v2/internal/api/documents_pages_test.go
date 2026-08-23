package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aladin/backend_v2/internal/app"
	coreservice "aladin/backend_v2/internal/service"
)

type fakeDocumentsSvc struct {
	pagesErr  error
	gotFrom   int
	gotTo     int
	gotID     string
	pageTexts []coreservice.DocumentPage
}

func (f *fakeDocumentsSvc) Get(context.Context, string, bool) (coreservice.Document, error) {
	return coreservice.Document{}, nil
}
func (f *fakeDocumentsSvc) Pages(_ context.Context, id string, from, to int) ([]coreservice.DocumentPage, error) {
	f.gotID, f.gotFrom, f.gotTo = id, from, to
	return f.pageTexts, f.pagesErr
}
func (f *fakeDocumentsSvc) Search(context.Context, string, string, int) ([]coreservice.DocumentHit, error) {
	return nil, nil
}
func (f *fakeDocumentsSvc) Regions(context.Context, string, string) ([]coreservice.DocumentRegion, error) {
	return nil, nil
}
func (f *fakeDocumentsSvc) Outline(context.Context, string) ([]coreservice.DocumentChunk, error) {
	return nil, nil
}

func pagesReq(query string) *http.Request {
	req := authedReq(http.MethodGet, "/api/artifacts/a1/document/pages"+query, "")
	req.SetPathValue("id", "a1")
	return req
}

func TestDocumentPagesHappyPath(t *testing.T) {
	t.Parallel()
	svc := &fakeDocumentsSvc{pageTexts: []coreservice.DocumentPage{{Page: 94, Text: "the collar's width"}}}
	s := &Server{deps: app.StaticDependencies{DocumentsSvc: svc}}

	rec := httptest.NewRecorder()
	s.handleArtifactDocumentPages(rec, pagesReq("?from=94&to=94"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.gotID != "a1" || svc.gotFrom != 94 || svc.gotTo != 94 {
		t.Fatalf("service got (%s, %d, %d), want (a1, 94, 94)", svc.gotID, svc.gotFrom, svc.gotTo)
	}
	var body struct {
		Pages []coreservice.DocumentPage `json:"pages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Pages) != 1 || body.Pages[0].Page != 94 {
		t.Fatalf("pages = %+v", body.Pages)
	}
}

func TestDocumentPagesClampsRange(t *testing.T) {
	t.Parallel()
	svc := &fakeDocumentsSvc{}
	s := &Server{deps: app.StaticDependencies{DocumentsSvc: svc}}

	rec := httptest.NewRecorder()
	s.handleArtifactDocumentPages(rec, pagesReq("?from=1&to=500"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.gotFrom != 1 || svc.gotTo != maxDocumentPageRange {
		t.Fatalf("service got (%d, %d), want range clamped to %d pages", svc.gotFrom, svc.gotTo, maxDocumentPageRange)
	}
}

func TestDocumentPagesRejectsBadParams(t *testing.T) {
	t.Parallel()
	s := &Server{deps: app.StaticDependencies{DocumentsSvc: &fakeDocumentsSvc{}}}

	for _, query := range []string{"", "?from=0&to=3", "?from=5&to=4", "?from=x&to=2"} {
		rec := httptest.NewRecorder()
		s.handleArtifactDocumentPages(rec, pagesReq(query))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("query %q: status = %d, want 400", query, rec.Code)
		}
	}
}

func TestDocumentPagesNotFoundMaps404(t *testing.T) {
	t.Parallel()
	svc := &fakeDocumentsSvc{pagesErr: coreservice.ErrNotFound}
	s := &Server{deps: app.StaticDependencies{DocumentsSvc: svc}}

	rec := httptest.NewRecorder()
	s.handleArtifactDocumentPages(rec, pagesReq("?from=1&to=1"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}
