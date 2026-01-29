package transactions

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
)

func setupTestDB(t *testing.T) *database.DB {
	t.Helper()

	dbPath := t.TempDir() + "/test.db"
	cfg := &config.DatabaseConfig{
		Path: dbPath,
	}

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	db.DB.SetMaxOpenConns(10)
	db.DB.SetMaxIdleConns(5)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestManager_BeginCommit(t *testing.T) {
	db := setupTestDB(t)
	manager := NewManager(db)
	defer manager.Close()

	ctx := context.Background()

	txID, expiresAt, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if txID == "" {
		t.Fatal("Begin returned empty transaction ID")
	}

	if !time.Now().Before(expiresAt) {
		t.Fatal("expiresAt should be in the future")
	}

	tx, err := manager.Get(txID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if tx == nil {
		t.Fatal("Get returned nil transaction")
	}

	if err := manager.Commit(ctx, txID); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if _, err := manager.Get(txID); err == nil {
		t.Fatal("Get should fail after commit")
	}
}

func TestManager_BeginRollback(t *testing.T) {
	db := setupTestDB(t)
	manager := NewManager(db)
	defer manager.Close()

	ctx := context.Background()

	txID, _, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if err := manager.Rollback(ctx, txID); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	if _, err := manager.Get(txID); err == nil {
		t.Fatal("Get should fail after rollback")
	}
}

func TestManager_Timeout(t *testing.T) {
	db := setupTestDB(t)

	os.Setenv("ALYX_TRANSACTION_TIMEOUT", "100ms")
	defer os.Unsetenv("ALYX_TRANSACTION_TIMEOUT")

	manager := NewManager(db)
	defer manager.Close()

	ctx := context.Background()

	txID, _, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	if _, err := manager.Get(txID); err == nil {
		t.Fatal("Get should fail after timeout")
	}
}

func TestManager_DoubleCommit(t *testing.T) {
	db := setupTestDB(t)
	manager := NewManager(db)
	defer manager.Close()

	ctx := context.Background()

	txID, _, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if err := manager.Commit(ctx, txID); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if err := manager.Commit(ctx, txID); err == nil {
		t.Fatal("Double commit should fail")
	}
}

func TestManager_DoubleRollback(t *testing.T) {
	db := setupTestDB(t)
	manager := NewManager(db)
	defer manager.Close()

	ctx := context.Background()

	txID, _, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	if err := manager.Rollback(ctx, txID); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	if err := manager.Rollback(ctx, txID); err == nil {
		t.Fatal("Double rollback should fail")
	}
}

func TestManager_Stats(t *testing.T) {
	db := setupTestDB(t)
	manager := NewManager(db)
	defer manager.Close()

	ctx := context.Background()

	stats := manager.Stats()
	if count, ok := stats["active_count"].(int); !ok || count != 0 {
		t.Fatalf("Expected 0 active transactions, got %v", stats["active_count"])
	}

	txID, _, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	stats = manager.Stats()
	if count, ok := stats["active_count"].(int); !ok || count != 1 {
		t.Fatalf("Expected 1 active transaction, got %v", stats["active_count"])
	}

	manager.Commit(ctx, txID)

	stats = manager.Stats()
	if count, ok := stats["active_count"].(int); !ok || count != 0 {
		t.Fatalf("Expected 0 active transactions after commit, got %v", stats["active_count"])
	}
}

func TestManager_Close(t *testing.T) {
	db := setupTestDB(t)
	manager := NewManager(db)

	ctx := context.Background()

	txID1, _, _ := manager.Begin(ctx)
	txID2, _, _ := manager.Begin(ctx)

	stats := manager.Stats()
	if count, ok := stats["active_count"].(int); !ok || count != 2 {
		t.Fatalf("Expected 2 active transactions, got %v", stats["active_count"])
	}

	if err := manager.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if _, err := manager.Get(txID1); err == nil {
		t.Fatal("Get should fail after Close")
	}

	if _, err := manager.Get(txID2); err == nil {
		t.Fatal("Get should fail after Close")
	}
}

func TestContextHelpers(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback()

	newCtx := database.WithTransaction(ctx, tx)

	retrieved, ok := database.TransactionFromContext(newCtx)
	if !ok {
		t.Fatal("TransactionFromContext returned false")
	}

	if retrieved != tx {
		t.Fatal("Retrieved transaction does not match original")
	}

	_, ok = database.TransactionFromContext(ctx)
	if ok {
		t.Fatal("TransactionFromContext should return false for context without transaction")
	}
}
