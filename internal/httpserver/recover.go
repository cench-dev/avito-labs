package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error(
						"panic recovered",
						"error", recovered,
						"stack", string(debug.Stack()),
					)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"code": "internal_error",
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
