package webhooks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRetryWorker_EnqueueWebhook(t *testing.T) {
	db := testDB(t)
	worker := NewRetryWorker(db, DefaultRetryConfig())

	tests := []struct {
		name        string
		webhookID   string
		endpointURL string
		payload     string
		headers     map[string]string
		wantErr     bool
	}{
		{
			name:        "basic enqueue",
			webhookID:   "webhook-1",
			endpointURL: "https://example.com/webhook",
			payload:     `{"event":"test"}`,
			headers:     map[string]string{"X-Custom": "value"},
			wantErr:     false,
		},
		{
			name:        "enqueue without headers",
			webhookID:   "webhook-2",
			endpointURL: "https://example.com/webhook2",
			payload:     `{"event":"test2"}`,
			headers:     nil,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := worker.EnqueueWebhook(ctx, tt.webhookID, tt.endpointURL, tt.payload, tt.headers)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnqueueWebhook() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				query := `SELECT COUNT(*) FROM _alyx_webhook_queue WHERE webhook_id = ? AND endpoint_url = ?`
				var count int
				err := db.QueryRowContext(ctx, query, tt.webhookID, tt.endpointURL).Scan(&count)
				if err != nil {
					t.Fatalf("Failed to query queue: %v", err)
				}
				if count != 1 {
					t.Errorf("Expected 1 queued webhook, got %d", count)
				}
			}
		})
	}
}

func TestRetryWorker_CalculateNextRetry(t *testing.T) {
	worker := NewRetryWorker(testDB(t), RetryConfig{
		BaseDelay: 1 * time.Second,
	})

	tests := []struct {
		name     string
		attempt  int
		expected time.Duration
	}{
		{"attempt 0", 0, 1 * time.Second},
		{"attempt 1", 1, 2 * time.Second},
		{"attempt 2", 2, 4 * time.Second},
		{"attempt 3", 3, 8 * time.Second},
		{"attempt 4", 4, 16 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now().UTC()
			nextRetry := worker.calculateNextRetry(tt.attempt)
			after := time.Now().UTC()

			minExpected := before.Add(tt.expected)
			maxExpected := after.Add(tt.expected)

			if nextRetry.Before(minExpected) || nextRetry.After(maxExpected) {
				t.Errorf("calculateNextRetry(%d) = %v, want between %v and %v",
					tt.attempt, nextRetry, minExpected, maxExpected)
			}
		})
	}
}

func TestRetryWorker_SuccessfulDelivery(t *testing.T) {
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer successServer.Close()

	db := testDB(t)
	worker := NewRetryWorker(db, DefaultRetryConfig())
	ctx := context.Background()

	err := worker.EnqueueWebhook(ctx, "webhook-1", successServer.URL, `{"event":"test"}`, nil)
	if err != nil {
		t.Fatalf("Failed to enqueue webhook: %v", err)
	}

	err = worker.processQueue()
	if err != nil {
		t.Fatalf("Failed to process queue: %v", err)
	}

	query := `SELECT status FROM _alyx_webhook_queue WHERE webhook_id = ?`
	var status string
	err = db.QueryRowContext(ctx, query, "webhook-1").Scan(&status)
	if err != nil {
		t.Fatalf("Failed to query status: %v", err)
	}

	if status != "succeeded" {
		t.Errorf("Expected status 'succeeded', got %q", status)
	}
}

func TestRetryWorker_FailureWithRetry(t *testing.T) {
	failureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer failureServer.Close()

	db := testDB(t)
	config := RetryConfig{
		MaxAttempts:  3,
		BaseDelay:    100 * time.Millisecond,
		PollInterval: 50 * time.Millisecond,
	}
	worker := NewRetryWorker(db, config)
	ctx := context.Background()

	err := worker.EnqueueWebhook(ctx, "webhook-1", failureServer.URL, `{"event":"test"}`, nil)
	if err != nil {
		t.Fatalf("Failed to enqueue webhook: %v", err)
	}

	for i := 0; i < config.MaxAttempts; i++ {
		err = worker.processQueue()
		if err != nil {
			t.Fatalf("Failed to process queue (attempt %d): %v", i+1, err)
		}

		query := `SELECT attempt, status FROM _alyx_webhook_queue WHERE webhook_id = ?`
		var attempt int
		var status string
		err = db.QueryRowContext(ctx, query, "webhook-1").Scan(&attempt, &status)
		if err != nil {
			t.Fatalf("Failed to query attempt: %v", err)
		}

		if i < config.MaxAttempts-1 {
			if status != "retrying" {
				t.Errorf("Attempt %d: expected status 'retrying', got %q", i+1, status)
			}
			if attempt != i+1 {
				t.Errorf("Attempt %d: expected attempt count %d, got %d", i+1, i+1, attempt)
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	query := `SELECT status FROM _alyx_webhook_queue WHERE webhook_id = ?`
	var status string
	err = db.QueryRowContext(ctx, query, "webhook-1").Scan(&status)
	if err != nil {
		t.Fatalf("Failed to query final status: %v", err)
	}

	if status != "failed" {
		t.Errorf("Expected final status 'failed', got %q", status)
	}
}

func TestRetryWorker_MoveToDLQ(t *testing.T) {
	failureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"bad gateway"}`))
	}))
	defer failureServer.Close()

	db := testDB(t)
	config := RetryConfig{
		MaxAttempts:  2,
		BaseDelay:    50 * time.Millisecond,
		PollInterval: 50 * time.Millisecond,
	}
	worker := NewRetryWorker(db, config)
	ctx := context.Background()

	err := worker.EnqueueWebhook(ctx, "webhook-1", failureServer.URL, `{"event":"test"}`, map[string]string{"X-Test": "value"})
	if err != nil {
		t.Fatalf("Failed to enqueue webhook: %v", err)
	}

	for i := 0; i < config.MaxAttempts; i++ {
		err = worker.processQueue()
		if err != nil {
			t.Fatalf("Failed to process queue (attempt %d): %v", i+1, err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	queryDLQ := `SELECT COUNT(*) FROM _alyx_webhook_dlq WHERE webhook_id = ?`
	var dlqCount int
	err = db.QueryRowContext(ctx, queryDLQ, "webhook-1").Scan(&dlqCount)
	if err != nil {
		t.Fatalf("Failed to query DLQ: %v", err)
	}

	if dlqCount != 1 {
		t.Errorf("Expected 1 entry in DLQ, got %d", dlqCount)
	}

	queryDLQDetails := `SELECT endpoint_url, attempts, last_error FROM _alyx_webhook_dlq WHERE webhook_id = ?`
	var endpointURL string
	var attempts int
	var lastError string
	err = db.QueryRowContext(ctx, queryDLQDetails, "webhook-1").Scan(&endpointURL, &attempts, &lastError)
	if err != nil {
		t.Fatalf("Failed to query DLQ details: %v", err)
	}

	if endpointURL != failureServer.URL {
		t.Errorf("Expected endpoint_url %q, got %q", failureServer.URL, endpointURL)
	}

	if attempts != config.MaxAttempts {
		t.Errorf("Expected %d attempts in DLQ, got %d", config.MaxAttempts, attempts)
	}

	if lastError == "" {
		t.Error("Expected last_error to be set in DLQ")
	}

	queryQueue := `SELECT status FROM _alyx_webhook_queue WHERE webhook_id = ?`
	var queueStatus string
	err = db.QueryRowContext(ctx, queryQueue, "webhook-1").Scan(&queueStatus)
	if err != nil {
		t.Fatalf("Failed to query queue status: %v", err)
	}

	if queueStatus != "failed" {
		t.Errorf("Expected queue status 'failed', got %q", queueStatus)
	}
}

func TestRetryWorker_StartStop(t *testing.T) {
	db := testDB(t)
	worker := NewRetryWorker(db, DefaultRetryConfig())

	ctx := context.Background()
	worker.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	worker.Stop()

	select {
	case <-worker.done:
	case <-time.After(1 * time.Second):
		t.Error("Worker did not stop within 1 second")
	}
}

func TestRetryWorker_ProcessQueueWithPendingRetries(t *testing.T) {
	db := testDB(t)
	worker := NewRetryWorker(db, DefaultRetryConfig())
	ctx := context.Background()

	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer successServer.Close()

	err := worker.EnqueueWebhook(ctx, "webhook-1", successServer.URL, `{"event":"test1"}`, nil)
	if err != nil {
		t.Fatalf("Failed to enqueue webhook 1: %v", err)
	}

	err = worker.EnqueueWebhook(ctx, "webhook-2", successServer.URL, `{"event":"test2"}`, nil)
	if err != nil {
		t.Fatalf("Failed to enqueue webhook 2: %v", err)
	}

	updateQuery := `UPDATE _alyx_webhook_queue SET next_retry_at = ? WHERE webhook_id = ?`
	futureTime := time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339)
	_, err = db.ExecContext(ctx, updateQuery, futureTime, "webhook-2")
	if err != nil {
		t.Fatalf("Failed to update next_retry_at: %v", err)
	}

	err = worker.processQueue()
	if err != nil {
		t.Fatalf("Failed to process queue: %v", err)
	}

	query := `SELECT status FROM _alyx_webhook_queue WHERE webhook_id = ?`

	var status1 string
	err = db.QueryRowContext(ctx, query, "webhook-1").Scan(&status1)
	if err != nil {
		t.Fatalf("Failed to query webhook-1 status: %v", err)
	}
	if status1 != "succeeded" {
		t.Errorf("Expected webhook-1 status 'succeeded', got %q", status1)
	}

	var status2 string
	err = db.QueryRowContext(ctx, query, "webhook-2").Scan(&status2)
	if err != nil {
		t.Fatalf("Failed to query webhook-2 status: %v", err)
	}
	if status2 != "pending" {
		t.Errorf("Expected webhook-2 status 'pending', got %q", status2)
	}
}
