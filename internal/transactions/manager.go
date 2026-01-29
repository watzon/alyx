package transactions

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/database"
)

// Manager manages transaction lifecycle with session-based tracking.
type Manager struct {
	db           *database.DB
	mu           sync.RWMutex
	transactions map[string]*activeTransaction
	timeout      time.Duration
}

// activeTransaction represents an active transaction with metadata.
type activeTransaction struct {
	tx        *sql.Tx
	createdAt time.Time
	timer     *time.Timer
}

// NewManager creates a new transaction manager.
func NewManager(db *database.DB) *Manager {
	timeout := 5 * time.Minute
	if timeoutStr := os.Getenv("ALYX_TRANSACTION_TIMEOUT"); timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = d
		}
	}

	return &Manager{
		db:           db,
		transactions: make(map[string]*activeTransaction),
		timeout:      timeout,
	}
}

// Begin starts a new transaction and returns a transaction ID.
func (m *Manager) Begin(ctx context.Context) (string, time.Time, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("beginning transaction: %w", err)
	}

	txID := "tx_" + uuid.New().String()
	createdAt := time.Now()
	expiresAt := createdAt.Add(m.timeout)

	timer := time.AfterFunc(m.timeout, func() {
		if err := m.autoRollback(txID); err != nil {
			log.Error().Err(err).Str("tx_id", txID).Msg("Auto-rollback failed")
		}
	})

	m.mu.Lock()
	m.transactions[txID] = &activeTransaction{
		tx:        tx,
		createdAt: createdAt,
		timer:     timer,
	}
	m.mu.Unlock()

	log.Debug().
		Str("tx_id", txID).
		Time("expires_at", expiresAt).
		Msg("Transaction started")

	return txID, expiresAt, nil
}

// Commit commits a transaction by ID.
func (m *Manager) Commit(ctx context.Context, txID string) error {
	m.mu.Lock()
	active, ok := m.transactions[txID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("transaction not found or expired: %s", txID)
	}

	delete(m.transactions, txID)
	m.mu.Unlock()

	active.timer.Stop()

	if err := active.tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	log.Debug().Str("tx_id", txID).Msg("Transaction committed")
	return nil
}

// Rollback rolls back a transaction by ID.
func (m *Manager) Rollback(ctx context.Context, txID string) error {
	m.mu.Lock()
	active, ok := m.transactions[txID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("transaction not found or expired: %s", txID)
	}

	delete(m.transactions, txID)
	m.mu.Unlock()

	active.timer.Stop()

	if err := active.tx.Rollback(); err != nil {
		return fmt.Errorf("rolling back transaction: %w", err)
	}

	log.Debug().Str("tx_id", txID).Msg("Transaction rolled back")
	return nil
}

// Get retrieves a transaction by ID.
func (m *Manager) Get(txID string) (*sql.Tx, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active, ok := m.transactions[txID]
	if !ok {
		return nil, fmt.Errorf("transaction not found or expired: %s", txID)
	}

	return active.tx, nil
}

// autoRollback automatically rolls back an expired transaction.
func (m *Manager) autoRollback(txID string) error {
	m.mu.Lock()
	active, ok := m.transactions[txID]
	if !ok {
		m.mu.Unlock()
		return nil
	}

	delete(m.transactions, txID)
	m.mu.Unlock()

	if err := active.tx.Rollback(); err != nil {
		return fmt.Errorf("auto-rolling back transaction: %w", err)
	}

	log.Warn().
		Str("tx_id", txID).
		Dur("age", time.Since(active.createdAt)).
		Msg("Transaction auto-rolled back due to timeout")

	return nil
}

// Stats returns transaction statistics.
func (m *Manager) Stats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]any{
		"active_count": len(m.transactions),
		"timeout":      m.timeout.String(),
	}
}

// Close cleans up all active transactions (rolls them back).
func (m *Manager) Close() error {
	m.mu.Lock()
	transactions := make([]*activeTransaction, 0, len(m.transactions))
	for _, active := range m.transactions {
		transactions = append(transactions, active)
	}
	m.transactions = make(map[string]*activeTransaction)
	m.mu.Unlock()

	for _, active := range transactions {
		active.timer.Stop()
		if err := active.tx.Rollback(); err != nil {
			log.Error().Err(err).Msg("Failed to rollback during close")
		}
	}

	return nil
}
