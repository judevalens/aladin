package httptransport

import (
	"net/http"

	"aladin/backend_v2/internal/httpapi"
	"aladin/backend_v2/internal/system"
)

type routes struct{ service system.SystemService }

func Register(mux *http.ServeMux, service system.SystemService) {
	r := routes{service: service}
	mux.HandleFunc("GET /api/worker/status", r.handleWorkerStatus)
	mux.HandleFunc("GET /api/pipeline/stats", r.handlePipelineStats)
}

func (h routes) handleWorkerStatus(w http.ResponseWriter, r *http.Request) {
	out, err := h.service.WorkerStatus(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h routes) handlePipelineStats(w http.ResponseWriter, r *http.Request) {
	out, err := h.service.PipelineStats(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "service_error", err.Error(), err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}
