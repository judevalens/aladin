package api

import (
	"net/http"

	coreservice "aladin/backend_v2/internal/service"
)

func (s *Server) handleInsightsList(w http.ResponseWriter, r *http.Request) {
	params := coreservice.InsightListParams{
		Limit:  min(intQuery(r, "limit", 30), 100),
		Offset: intQuery(r, "offset", 0),
		Type:   r.URL.Query().Get("type"),
		Status: firstNonEmpty(r.URL.Query().Get("status"), "pending"),
	}
	out, err := s.deps.Insights().List(r.Context(), params)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleInsightAccept(w http.ResponseWriter, r *http.Request) {
	s.updateInsightStatus(w, r, "accepted")
}
func (s *Server) handleInsightDismiss(w http.ResponseWriter, r *http.Request) {
	s.updateInsightStatus(w, r, "dismissed")
}

func (s *Server) updateInsightStatus(w http.ResponseWriter, r *http.Request, status string) {
	id := r.PathValue("id")
	if err := s.deps.Insights().UpdateStatus(r.Context(), id, status); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleInsightsStats(w http.ResponseWriter, r *http.Request) {
	out, _ := s.deps.Insights().Stats(r.Context())
	writeJSON(w, http.StatusOK, out)
}
