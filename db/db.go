package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a sql.DB connection to the SQLite database.
type DB struct {
	conn *sql.DB
}

// New opens the SQLite database at the given path, runs migrations, and returns a DB.
func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	conn.SetMaxOpenConns(10) // WAL mode allows concurrent readers with one writer

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying sql.DB for use in other packages.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS admin (
		id            INTEGER PRIMARY KEY CHECK (id = 1),
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS users (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		name           TEXT NOT NULL,
		api_key_hash   TEXT NOT NULL UNIQUE,
		api_key_prefix TEXT NOT NULL,
		active         BOOLEAN DEFAULT 1,
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS usage_logs (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		model             TEXT,
		path              TEXT,
		prompt_tokens     INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		total_tokens      INTEGER DEFAULT 0,
		created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_usage_logs_user_time ON usage_logs(user_id, created_at);
	`
	_, err := db.conn.Exec(schema)
	return err
}

// StartCleanupJob starts a goroutine that deletes usage_logs older than 30 days every hour.
func (db *DB) StartCleanupJob() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			result, err := db.conn.Exec("DELETE FROM usage_logs WHERE created_at < datetime('now', '-30 days')")
			if err != nil {
				log.Printf("ERROR: cleanup job failed: %v", err)
				continue
			}
			if rows, _ := result.RowsAffected(); rows > 0 {
				log.Printf("Cleanup: deleted %d old usage logs", rows)
			}
		}
	}()
}
