// Package httpapi owns transport primitives shared by domain HTTP adapters.
// Domain packages map their own errors but keep response and request tracing
// behavior consistent through this package.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"aladin/backend_v2/internal/service"
)

type requestIDKey struct{}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func RequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

func PrincipalUserID(r *http.Request) string {
	if principal, ok := service.PrincipalFromContext(r.Context()); ok {
		return principal.UserID
	}
	return ""
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func ReadJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("missing request body")
	}
	return json.NewDecoder(r.Body).Decode(dst)
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, category, publicMessage string, err error) {
	attrs := []any{
		"component", "api",
		"request_id", RequestID(r.Context()),
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"category", category,
	}
	if err != nil {
		attrs = append(attrs, "err", err)
	} else {
		attrs = append(attrs, "message", publicMessage)
	}
	slog.Error("api: request failed", attrs...)
	WriteJSON(w, status, map[string]string{"error": publicMessage})
}

func WriteDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error(
		"api: request decode failed",
		"component", "api",
		"request_id", RequestID(r.Context()),
		"method", r.Method,
		"path", r.URL.Path,
		"status", http.StatusBadRequest,
		"category", "decode_error",
		"content_type", r.Header.Get("Content-Type"),
		"err", err,
	)
	WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
}
