package mcpserver

import (
	"net/http"

	"aladin/backend_v2/internal/service"
)

func bearerAuth(auth service.AuthService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := service.ResolveBearerPrincipal(r.Context(), auth, r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(service.WithPrincipal(r.Context(), principal)))
	})
}
