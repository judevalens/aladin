package api

import (
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) handleFeedList(w http.ResponseWriter, r *http.Request) {
	params := coreservice.FeedListParams{
		Limit:      min(intQuery(r, "limit", 50), 200),
		Offset:     intQuery(r, "offset", 0),
		SourceType: r.URL.Query().Get("source_type"),
		Topic:      r.URL.Query().Get("topic"),
		SavedOnly:  r.URL.Query().Get("saved") == "true",
		Sort:       firstNonEmpty(r.URL.Query().Get("sort"), "recent"),
	}
	out, err := s.deps.Feed().List(r.Context(), params)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleFeedTopics(w http.ResponseWriter, r *http.Request) {
	out, _ := s.deps.Feed().Topics(r.Context())
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleFeedSources(w http.ResponseWriter, r *http.Request) {
	out, _ := s.deps.Feed().Sources(r.Context())
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleFeedSave(w http.ResponseWriter, r *http.Request) {
	s.updateRecordUserStatus(w, r, "saved")
}
func (s *Server) handleFeedDismiss(w http.ResponseWriter, r *http.Request) {
	s.updateRecordUserStatus(w, r, "dismissed")
}
func (s *Server) handleFeedUnsave(w http.ResponseWriter, r *http.Request) {
	s.updateRecordUserStatus(w, r, "")
}

func (s *Server) updateRecordUserStatus(w http.ResponseWriter, r *http.Request, status string) {
	id := r.PathValue("id")
	if err := s.deps.Feed().UpdateStatus(r.Context(), id, status); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
