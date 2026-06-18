package api

import "net/http"

func (s *Server) handleWorkerStatus(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.System().WorkerStatus(r.Context())
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePipelineStats(w http.ResponseWriter, r *http.Request) {
	out, err := s.deps.System().PipelineStats(r.Context())
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, categoryServiceError, err.Error(), err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
