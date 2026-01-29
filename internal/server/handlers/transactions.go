package handlers

import (
	"net/http"

	"github.com/watzon/alyx/internal/transactions"
)

type TransactionHandlers struct {
	manager *transactions.Manager
}

func NewTransactionHandlers(manager *transactions.Manager) *TransactionHandlers {
	return &TransactionHandlers{
		manager: manager,
	}
}

func (h *TransactionHandlers) Begin(w http.ResponseWriter, r *http.Request) {
	txID, expiresAt, err := h.manager.Begin(r.Context())
	if err != nil {
		InternalError(w, "Failed to begin transaction")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"transaction_id": txID,
		"expires_at":     expiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *TransactionHandlers) Commit(w http.ResponseWriter, r *http.Request) {
	txID := r.URL.Query().Get("tx_id")
	if txID == "" {
		Error(w, http.StatusBadRequest, "MISSING_TX_ID", "Transaction ID is required")
		return
	}

	if err := h.manager.Commit(r.Context(), txID); err != nil {
		Error(w, http.StatusBadRequest, "COMMIT_FAILED", err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"status": "committed",
	})
}

func (h *TransactionHandlers) Rollback(w http.ResponseWriter, r *http.Request) {
	txID := r.URL.Query().Get("tx_id")
	if txID == "" {
		Error(w, http.StatusBadRequest, "MISSING_TX_ID", "Transaction ID is required")
		return
	}

	if err := h.manager.Rollback(r.Context(), txID); err != nil {
		Error(w, http.StatusBadRequest, "ROLLBACK_FAILED", err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"status": "rolled_back",
	})
}
