package api

import "net/http"

func (s *Server) handleWorkerStatus(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.System.WorkerStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}
