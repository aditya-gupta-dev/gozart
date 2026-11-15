package storage

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Checkpoint state constants
const (
	StateInit            = "INIT"
	StateResearchDone    = "RESEARCH_DONE"
	StateSentimentDone   = "SENTIMENT_DONE"
	StateScriptDone      = "SCRIPT_DONE"
	StateParallelGenDone = "PARALLEL_GEN_DONE"
	StateComposited      = "COMPOSITED"
	StateUploaded        = "UPLOADED"
	StateFailed          = "FAILED"
)

// Status constants
const (
	StatusQueued    = "QUEUED"
	StatusRunning   = "RUNNING"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"
)

type DB struct {
	db *sql.DB
	mu sync.RWMutex
}

// New initializes an embedded SQLite database with WAL mode and schema migration.
func New(dbPath string) (*DB, error) {
	if dbPath == "" {
		dbPath = "gozart.db"
	}

	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Single writer / connection pool settings for SQLite
	db.SetMaxOpenConns(1)

	// Execute migrations
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to execute schema migration: %w", err)
	}

	return &DB{db: db}, nil
}

func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *DB) SQLDB() *sql.DB {
	return d.db
}
