package db

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// User represents a proxy user.
type User struct {
	ID           int
	Name         string
	APIKeyPrefix string
	Active       bool
	CreatedAt    time.Time
}

// CreateUser creates a new user and returns the user and the plaintext API key (shown once).
func (db *DB) CreateUser(name string) (*User, string, error) {
	apiKey := "sk-" + uuid.New().String()
	hash := hashAPIKey(apiKey)
	prefix := apiKey[:11] // e.g., "sk-550e8400"

	result, err := db.conn.Exec(
		"INSERT INTO users (name, api_key_hash, api_key_prefix, active) VALUES (?, ?, ?, 1)",
		name, hash, prefix,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	id, _ := result.LastInsertId()
	user := &User{
		ID:           int(id),
		Name:         name,
		APIKeyPrefix: prefix,
		Active:       true,
		CreatedAt:    time.Now(),
	}
	return user, apiKey, nil
}

// GetUserByAPIKey looks up a user by their API key (hashed).
// Returns nil if not found.
func (db *DB) GetUserByAPIKey(apiKey string) (*User, error) {
	hash := hashAPIKey(apiKey)
	var u User
	var active int
	err := db.conn.QueryRow(
		"SELECT id, name, api_key_prefix, active, created_at FROM users WHERE api_key_hash = ?",
		hash,
	).Scan(&u.ID, &u.Name, &u.APIKeyPrefix, &active, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.Active = active == 1
	return &u, nil
}

// ListUsers returns all users.
func (db *DB) ListUsers() ([]User, error) {
	rows, err := db.conn.Query(
		"SELECT id, name, api_key_prefix, active, created_at FROM users ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var active int
		if err := rows.Scan(&u.ID, &u.Name, &u.APIKeyPrefix, &active, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Active = active == 1
		users = append(users, u)
	}
	return users, rows.Err()
}

// DeleteUser deletes a user by ID (cascade deletes usage_logs).
func (db *DB) DeleteUser(id int) error {
	result, err := db.conn.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// SetUserActive sets the active status of a user.
func (db *DB) SetUserActive(id int, active bool) error {
	val := 0
	if active {
		val = 1
	}
	result, err := db.conn.Exec("UPDATE users SET active = ? WHERE id = ?", val, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// GetUser returns a single user by ID.
func (db *DB) GetUser(id int) (*User, error) {
	var u User
	var active int
	err := db.conn.QueryRow(
		"SELECT id, name, api_key_prefix, active, created_at FROM users WHERE id = ?", id,
	).Scan(&u.ID, &u.Name, &u.APIKeyPrefix, &active, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}
	u.Active = active == 1
	return &u, nil
}

func hashAPIKey(apiKey string) string {
	h := sha256.Sum256([]byte(apiKey))
	return fmt.Sprintf("%x", h)
}
