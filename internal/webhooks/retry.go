package webhooks

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/database"
)

type RetryConfig struct {
	MaxAttempts  int
	BaseDelay    time.Duration
	PollInterval time.Duration
}

type RetryWorker struct {
	db         *database.DB
	config     RetryConfig
	httpClient *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
}

type QueuedWebhook struct {
	ID          string
	WebhookID   string
	EndpointURL string
	Payload     string
	Headers     map[string]string
	Attempt     int
	NextRetryAt *time.Time
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  5,
		BaseDelay:    1 * time.Second,
		PollInterval: 5 * time.Second,
	}
}

func NewRetryWorker(db *database.DB, config RetryConfig) *RetryWorker {
	ctx, cancel := context.WithCancel(context.Background())

	return &RetryWorker{
		db:     db,
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
}

func (w *RetryWorker) Start(ctx context.Context) {
	log.Info().
		Int("max_attempts", w.config.MaxAttempts).
		Dur("base_delay", w.config.BaseDelay).
		Dur("poll_interval", w.config.PollInterval).
		Msg("Starting webhook retry worker")

	go w.run()
}

func (w *RetryWorker) Stop() {
	log.Info().Msg("Stopping webhook retry worker")
	w.cancel()
	<-w.done
}

func (w *RetryWorker) run() {
	defer close(w.done)

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if err := w.processQueue(); err != nil {
				log.Error().Err(err).Msg("Error processing webhook retry queue")
			}
		}
	}
}

func (w *RetryWorker) processQueue() error {
	query := `
		SELECT id, webhook_id, endpoint_url, payload, headers, attempt, next_retry_at, 
		       status, created_at, updated_at
		FROM _alyx_webhook_queue
		WHERE status IN ('pending', 'retrying')
		  AND (next_retry_at IS NULL OR next_retry_at <= ?)
		ORDER BY created_at ASC
		LIMIT 100
	`

	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := w.db.QueryContext(w.ctx, query, now)
	if err != nil {
		return fmt.Errorf("querying webhook queue: %w", err)
	}
	defer rows.Close()

	webhooks, err := w.scanQueuedWebhooks(rows)
	if err != nil {
		return fmt.Errorf("scanning queued webhooks: %w", err)
	}

	for _, webhook := range webhooks {
		if err := w.retryWebhook(webhook); err != nil {
			log.Error().
				Err(err).
				Str("id", webhook.ID).
				Str("endpoint", webhook.EndpointURL).
				Int("attempt", webhook.Attempt).
				Msg("Failed to retry webhook")
		}
	}

	return nil
}

func (w *RetryWorker) retryWebhook(webhook *QueuedWebhook) error {
	log.Debug().
		Str("id", webhook.ID).
		Str("endpoint", webhook.EndpointURL).
		Int("attempt", webhook.Attempt+1).
		Msg("Retrying webhook delivery")

	req, err := http.NewRequestWithContext(w.ctx, http.MethodPost, webhook.EndpointURL, bytes.NewReader([]byte(webhook.Payload)))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return w.handleRetryFailure(webhook, fmt.Sprintf("HTTP request failed: %v", err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return w.markSuccess(webhook)
	}

	errorMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
	return w.handleRetryFailure(webhook, errorMsg)
}

func (w *RetryWorker) handleRetryFailure(webhook *QueuedWebhook, errorMsg string) error {
	webhook.Attempt++

	if webhook.Attempt >= w.config.MaxAttempts {
		log.Warn().
			Str("id", webhook.ID).
			Str("endpoint", webhook.EndpointURL).
			Int("attempts", webhook.Attempt).
			Msg("Webhook exceeded max attempts, moving to DLQ")

		return w.moveToDLQ(webhook, errorMsg)
	}

	nextRetry := w.calculateNextRetry(webhook.Attempt)
	query := `
		UPDATE _alyx_webhook_queue
		SET attempt = ?, next_retry_at = ?, status = 'retrying', updated_at = ?
		WHERE id = ?
	`

	now := time.Now().UTC()
	_, err := w.db.ExecContext(w.ctx, query,
		webhook.Attempt,
		nextRetry.Format(time.RFC3339),
		now.Format(time.RFC3339),
		webhook.ID,
	)
	if err != nil {
		return fmt.Errorf("updating webhook queue: %w", err)
	}

	log.Debug().
		Str("id", webhook.ID).
		Int("attempt", webhook.Attempt).
		Time("next_retry", nextRetry).
		Msg("Scheduled webhook for retry")

	return nil
}

func (w *RetryWorker) markSuccess(webhook *QueuedWebhook) error {
	log.Info().
		Str("id", webhook.ID).
		Str("endpoint", webhook.EndpointURL).
		Int("attempt", webhook.Attempt+1).
		Msg("Webhook delivery succeeded")

	query := `
		UPDATE _alyx_webhook_queue
		SET status = 'succeeded', updated_at = ?
		WHERE id = ?
	`

	now := time.Now().UTC()
	_, err := w.db.ExecContext(w.ctx, query, now.Format(time.RFC3339), webhook.ID)
	if err != nil {
		return fmt.Errorf("marking webhook success: %w", err)
	}

	return nil
}

func (w *RetryWorker) moveToDLQ(webhook *QueuedWebhook, errorMsg string) error {
	tx, err := w.db.BeginTx(w.ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	insertDLQ := `
		INSERT INTO _alyx_webhook_dlq (id, webhook_id, endpoint_url, payload, headers, attempts, last_error, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	headersJSON, err := json.Marshal(webhook.Headers)
	if err != nil {
		return fmt.Errorf("marshaling headers: %w", err)
	}

	dlqID := uuid.New().String()
	now := time.Now().UTC()

	_, err = tx.ExecContext(w.ctx, insertDLQ,
		dlqID,
		webhook.WebhookID,
		webhook.EndpointURL,
		webhook.Payload,
		string(headersJSON),
		webhook.Attempt,
		errorMsg,
		now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting into DLQ: %w", err)
	}

	updateQueue := `
		UPDATE _alyx_webhook_queue
		SET status = 'failed', updated_at = ?
		WHERE id = ?
	`

	_, err = tx.ExecContext(w.ctx, updateQueue, now.Format(time.RFC3339), webhook.ID)
	if err != nil {
		return fmt.Errorf("updating queue status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	log.Info().
		Str("id", webhook.ID).
		Str("dlq_id", dlqID).
		Str("endpoint", webhook.EndpointURL).
		Int("attempts", webhook.Attempt).
		Msg("Moved webhook to DLQ")

	return nil
}

func (w *RetryWorker) calculateNextRetry(attempt int) time.Time {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > 30 {
		attempt = 30
	}
	delay := w.config.BaseDelay * time.Duration(1<<attempt)
	return time.Now().UTC().Add(delay)
}

func (w *RetryWorker) EnqueueWebhook(ctx context.Context, webhookID, endpointURL, payload string, headers map[string]string) error {
	query := `
		INSERT INTO _alyx_webhook_queue (id, webhook_id, endpoint_url, payload, headers, attempt, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, 'pending', ?, ?)
	`

	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("marshaling headers: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = w.db.ExecContext(ctx, query,
		id,
		webhookID,
		endpointURL,
		payload,
		string(headersJSON),
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("enqueueing webhook: %w", err)
	}

	log.Debug().
		Str("id", id).
		Str("webhook_id", webhookID).
		Str("endpoint", endpointURL).
		Msg("Enqueued webhook for retry")

	return nil
}

func (w *RetryWorker) scanQueuedWebhooks(rows *sql.Rows) ([]*QueuedWebhook, error) {
	var webhooks []*QueuedWebhook

	for rows.Next() {
		var webhook QueuedWebhook
		var headersJSON sql.NullString
		var nextRetryAt sql.NullString
		var createdAt, updatedAt string

		err := rows.Scan(
			&webhook.ID,
			&webhook.WebhookID,
			&webhook.EndpointURL,
			&webhook.Payload,
			&headersJSON,
			&webhook.Attempt,
			&nextRetryAt,
			&webhook.Status,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if headersJSON.Valid && headersJSON.String != "" {
			if err := json.Unmarshal([]byte(headersJSON.String), &webhook.Headers); err != nil {
				return nil, fmt.Errorf("unmarshaling headers: %w", err)
			}
		}

		if nextRetryAt.Valid {
			t, err := time.Parse(time.RFC3339, nextRetryAt.String)
			if err != nil {
				return nil, fmt.Errorf("parsing next_retry_at: %w", err)
			}
			webhook.NextRetryAt = &t
		}

		webhook.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		webhook.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		webhooks = append(webhooks, &webhook)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return webhooks, nil
}
