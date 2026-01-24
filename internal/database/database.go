package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database/migrations"
)

type DB struct {
	*sql.DB
	cfg    *config.DatabaseConfig
	mu     sync.RWMutex
	closed bool
}

func Open(cfg *config.DatabaseConfig) (*DB, error) {
	if err := ensureDir(cfg.Path); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	dsn := buildDSN(cfg)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db := &DB{
		DB:  sqlDB,
		cfg: cfg,
	}

	if err := db.configure(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("configuring database: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	if err := migrations.Run(context.Background(), sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

func buildDSN(cfg *config.DatabaseConfig) string {
	return cfg.Path
}

func ensureDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func (db *DB) configure() error {
	pragmas := []string{
		"PRAGMA busy_timeout = " + fmt.Sprintf("%d", db.cfg.BusyTimeout.Milliseconds()),
	}

	if db.cfg.WALMode {
		pragmas = append(pragmas, "PRAGMA journal_mode = WAL")
		pragmas = append(pragmas, "PRAGMA synchronous = NORMAL")
	}

	if db.cfg.ForeignKeys {
		pragmas = append(pragmas, "PRAGMA foreign_keys = ON")
	}

	if db.cfg.CacheSize != 0 {
		pragmas = append(pragmas, fmt.Sprintf("PRAGMA cache_size = %d", db.cfg.CacheSize))
	}

	pragmas = append(pragmas, "PRAGMA temp_store = MEMORY")

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("executing %q: %w", pragma, err)
		}
	}

	return nil
}

func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil
	}
	db.closed = true

	if db.cfg.WALMode {
		_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	}

	return db.DB.Close()
}

func (db *DB) Ping(ctx context.Context) error {
	return db.DB.PingContext(ctx)
}

func (db *DB) Transaction(ctx context.Context, fn func(tx *Tx) error) error {
	sqlTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	tx := &Tx{Tx: sqlTx}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %w (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

type Tx struct {
	*sql.Tx
}

func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.DB.ExecContext(ctx, query, args...)
}

func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.DB.QueryContext(ctx, query, args...)
}

func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return db.DB.QueryRowContext(ctx, query, args...)
}

func (db *DB) Stats() sql.DBStats {
	return db.DB.Stats()
}

type Row map[string]any

func ScanRows(rows *sql.Rows) ([]Row, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("getting columns: %w", err)
	}

	var results []Row

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		row := make(Row)
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return results, nil
}

func ScanRow(row *sql.Row, columns []string) (Row, error) {
	values := make([]any, len(columns))
	valuePtrs := make([]any, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := row.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	result := make(Row)
	for i, col := range columns {
		val := values[i]
		if b, ok := val.([]byte); ok {
			result[col] = string(b)
		} else {
			result[col] = val
		}
	}

	return result, nil
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
