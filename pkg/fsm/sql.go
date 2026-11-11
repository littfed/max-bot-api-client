package fsm

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SQLStorage implements persistent Storage backed by any standard database/sql database (SQLite, PostgreSQL, MySQL).
type SQLStorage struct {
	db        *sql.DB
	tableName string
	mu        sync.RWMutex
}

// SQLStorageOptions configures the SQLStorage instance.
type SQLStorageOptions struct {
	TableName string
}

// NewSQLStorage creates and initializes a persistent SQL storage instance.
// It automatically executes a CREATE TABLE IF NOT EXISTS query for the configured table.
func NewSQLStorage(db *sql.DB, opts ...SQLStorageOptions) (*SQLStorage, error) {
	if db == nil {
		return nil, fmt.Errorf("fsm: db cannot be nil")
	}

	tbl := "fsm_states"
	if len(opts) > 0 && opts[0].TableName != "" {
		tbl = opts[0].TableName
	}

	storage := &SQLStorage{
		db:        db,
		tableName: tbl,
	}

	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		user_id BIGINT NOT NULL,
		chat_id BIGINT NOT NULL,
		state TEXT NOT NULL,
		data TEXT NOT NULL,
		updated_at BIGINT NOT NULL,
		PRIMARY KEY (user_id, chat_id)
	);`, tbl)

	if _, err := db.Exec(query); err != nil {
		return nil, fmt.Errorf("fsm: failed to create table %s: %w", tbl, err)
	}

	return storage, nil
}

// SetState updates or inserts the state for the given session key.
func (s *SQLStorage) SetState(ctx context.Context, key SessionKey, state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()

	// Portable UPSERT check
	var dummy int
	checkQuery := fmt.Sprintf("SELECT 1 FROM %s WHERE user_id = ? AND chat_id = ?", s.tableName)
	err := s.db.QueryRowContext(ctx, checkQuery, key.UserID, key.ChatID).Scan(&dummy)

	if err == sql.ErrNoRows {
		insertQuery := fmt.Sprintf("INSERT INTO %s (user_id, chat_id, state, data, updated_at) VALUES (?, ?, ?, ?, ?)", s.tableName)
		_, err = s.db.ExecContext(ctx, insertQuery, key.UserID, key.ChatID, string(state), "{}", now)
		return err
	} else if err != nil {
		return err
	}

	updateQuery := fmt.Sprintf("UPDATE %s SET state = ?, updated_at = ? WHERE user_id = ? AND chat_id = ?", s.tableName)
	_, err = s.db.ExecContext(ctx, updateQuery, string(state), now, key.UserID, key.ChatID)
	return err
}

// GetState retrieves the current state for the given session key.
func (s *SQLStorage) GetState(ctx context.Context, key SessionKey) (State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stateStr string
	query := fmt.Sprintf("SELECT state FROM %s WHERE user_id = ? AND chat_id = ?", s.tableName)
	err := s.db.QueryRowContext(ctx, query, key.UserID, key.ChatID).Scan(&stateStr)
	if err == sql.ErrNoRows {
		return StateEmpty, nil
	}
	if err != nil {
		return StateEmpty, err
	}
	return State(stateStr), nil
}

// SetData stores serialized arbitrary JSON data for the given session key.
func (s *SQLStorage) SetData(ctx context.Context, key SessionKey, data map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("fsm: failed to marshal data: %w", err)
	}

	now := time.Now().Unix()
	var dummy int
	checkQuery := fmt.Sprintf("SELECT 1 FROM %s WHERE user_id = ? AND chat_id = ?", s.tableName)
	err = s.db.QueryRowContext(ctx, checkQuery, key.UserID, key.ChatID).Scan(&dummy)

	if err == sql.ErrNoRows {
		insertQuery := fmt.Sprintf("INSERT INTO %s (user_id, chat_id, state, data, updated_at) VALUES (?, ?, ?, ?, ?)", s.tableName)
		_, err = s.db.ExecContext(ctx, insertQuery, key.UserID, key.ChatID, string(StateEmpty), string(raw), now)
		return err
	} else if err != nil {
		return err
	}

	updateQuery := fmt.Sprintf("UPDATE %s SET data = ?, updated_at = ? WHERE user_id = ? AND chat_id = ?", s.tableName)
	_, err = s.db.ExecContext(ctx, updateQuery, string(raw), now, key.UserID, key.ChatID)
	return err
}

// GetData retrieves and deserializes the stored data for the given session key.
func (s *SQLStorage) GetData(ctx context.Context, key SessionKey) (map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rawData string
	query := fmt.Sprintf("SELECT data FROM %s WHERE user_id = ? AND chat_id = ?", s.tableName)
	err := s.db.QueryRowContext(ctx, query, key.UserID, key.ChatID).Scan(&rawData)
	if err == sql.ErrNoRows {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)
	if rawData == "" || rawData == "{}" {
		return result, nil
	}

	if err := json.Unmarshal([]byte(rawData), &result); err != nil {
		return nil, fmt.Errorf("fsm: failed to unmarshal data: %w", err)
	}
	return result, nil
}

// UpdateData retrieves the existing data, allows fn to mutate it in-place, and persists the result.
func (s *SQLStorage) UpdateData(ctx context.Context, key SessionKey, fn func(data map[string]any) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var rawData string
	query := fmt.Sprintf("SELECT data FROM %s WHERE user_id = ? AND chat_id = ?", s.tableName)
	err := s.db.QueryRowContext(ctx, query, key.UserID, key.ChatID).Scan(&rawData)

	data := make(map[string]any)
	if err == nil && rawData != "" && rawData != "{}" {
		_ = json.Unmarshal([]byte(rawData), &data)
	}

	if err := fn(data); err != nil {
		return err
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("fsm: failed to marshal updated data: %w", err)
	}

	now := time.Now().Unix()
	var dummy int
	checkQuery := fmt.Sprintf("SELECT 1 FROM %s WHERE user_id = ? AND chat_id = ?", s.tableName)
	err = s.db.QueryRowContext(ctx, checkQuery, key.UserID, key.ChatID).Scan(&dummy)

	if err == sql.ErrNoRows {
		insertQuery := fmt.Sprintf("INSERT INTO %s (user_id, chat_id, state, data, updated_at) VALUES (?, ?, ?, ?, ?)", s.tableName)
		_, err = s.db.ExecContext(ctx, insertQuery, key.UserID, key.ChatID, string(StateEmpty), string(raw), now)
		return err
	} else if err != nil {
		return err
	}

	updateQuery := fmt.Sprintf("UPDATE %s SET data = ?, updated_at = ? WHERE user_id = ? AND chat_id = ?", s.tableName)
	_, err = s.db.ExecContext(ctx, updateQuery, string(raw), now, key.UserID, key.ChatID)
	return err
}

// Reset clears the state and data for the session key.
func (s *SQLStorage) Reset(ctx context.Context, key SessionKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := fmt.Sprintf("DELETE FROM %s WHERE user_id = ? AND chat_id = ?", s.tableName)
	_, err := s.db.ExecContext(ctx, query, key.UserID, key.ChatID)
	return err
}

// Close closes the underlying database connection.
func (s *SQLStorage) Close() error {
	return s.db.Close()
}
