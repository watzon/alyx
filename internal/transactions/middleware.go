package transactions

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/watzon/alyx/internal/database"
)

// Middleware returns HTTP middleware that injects transactions into request context.
func Middleware(manager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			txID := r.URL.Query().Get("tx_id")
			if txID == "" {
				next.ServeHTTP(w, r)
				return
			}

			tx, err := manager.Get(txID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_TRANSACTION", err.Error())
				return
			}

			ctx := database.WithTransaction(r.Context(), tx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type errorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	Timestamp string `json:"timestamp"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error:     message,
		Code:      code,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
