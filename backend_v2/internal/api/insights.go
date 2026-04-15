package api

import "net/http"

type insightRecord struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Entity      *string  `json:"entity"`
	Topic       *string  `json:"topic"`
	ArtifactIDs []string `json:"artifactIds"`
	Confidence  float64  `json:"confidence"`
	UserStatus  string   `json:"userStatus"`
	CreatedAt   string   `json:"createdAt"`
}

func (s *Server) handleInsightsList(w http.ResponseWriter, r *http.Request) {
	params := insightListParams{
		Limit:  min(intQuery(r, "limit", 30), 100),
		Offset: intQuery(r, "offset", 0),
		Type:   r.URL.Query().Get("type"),
		Status: firstNonEmpty(r.URL.Query().Get("status"), "pending"),
	}
	out, err := s.deps.Insights.List(r.Context(), params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
	if err := s.deps.Insights.UpdateStatus(r.Context(), id, status); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleInsightsStats(w http.ResponseWriter, r *http.Request) {
	out, _ := s.deps.Insights.Stats(r.Context())
	writeJSON(w, http.StatusOK, out)
}
