package fsm

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"sync"
	"testing"
)

// In-memory SQL driver for testing SQLStorage without CGO or external dependencies
type mockSQLDriver struct {
	mu     sync.Mutex
	tables map[string]map[string]*mockRow
}

type mockRow struct {
	userID    int64
	chatID    int64
	state     string
	data      string
	updatedAt int64
}

var globalMockDriver = &mockSQLDriver{
	tables: make(map[string]map[string]*mockRow),
}

func init() {
	sql.Register("fsm_mock", globalMockDriver)
}

func (d *mockSQLDriver) Open(name string) (driver.Conn, error) {
	return &mockSQLConn{d: d}, nil
}

type mockSQLConn struct {
	d *mockSQLDriver
}

func (c *mockSQLConn) Prepare(query string) (driver.Stmt, error) {
	return &mockSQLStmt{query: query, conn: c}, nil
}

func (c *mockSQLConn) Close() error {
	return nil
}

func (c *mockSQLConn) Begin() (driver.Tx, error) {
	return nil, nil
}

type mockSQLStmt struct {
	query string
	conn  *mockSQLConn
}

func (s *mockSQLStmt) Close() error { return nil }
func (s *mockSQLStmt) NumInput() int { return -1 }

func (s *mockSQLStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.conn.d.mu.Lock()
	defer s.conn.d.mu.Unlock()

	table := "fsm_states"
	if _, ok := s.conn.d.tables[table]; !ok {
		s.conn.d.tables[table] = make(map[string]*mockRow)
	}

	// Simple query parser for testing
	if len(args) == 5 { // INSERT
		uid := args[0].(int64)
		cid := args[1].(int64)
		key := fmt.Sprintf("%d:%d", uid, cid)
		s.conn.d.tables[table][key] = &mockRow{
			userID:    uid,
			chatID:    cid,
			state:     args[2].(string),
			data:      args[3].(string),
			updatedAt: args[4].(int64),
		}
		return driver.RowsAffected(1), nil
	}

	if len(args) == 4 { // UPDATE
		// state, updated_at, user_id, chat_id OR data, updated_at, user_id, chat_id
		val := args[0].(string)
		updatedAt := args[1].(int64)
		uid := args[2].(int64)
		cid := args[3].(int64)
		key := fmt.Sprintf("%d:%d", uid, cid)
		row, ok := s.conn.d.tables[table][key]
		if ok {
			row.updatedAt = updatedAt
			if val[0] == '{' { // Data
				row.data = val
			} else {
				row.state = val
			}
		}
		return driver.RowsAffected(1), nil
	}

	if len(args) == 2 { // DELETE
		uid := args[0].(int64)
		cid := args[1].(int64)
		key := fmt.Sprintf("%d:%d", uid, cid)
		delete(s.conn.d.tables[table], key)
		return driver.RowsAffected(1), nil
	}

	return driver.RowsAffected(0), nil
}

func (s *mockSQLStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.conn.d.mu.Lock()
	defer s.conn.d.mu.Unlock()

	table := "fsm_states"
	rowsMap := s.conn.d.tables[table]

	if len(args) == 2 {
		uid := args[0].(int64)
		cid := args[1].(int64)
		key := fmt.Sprintf("%d:%d", uid, cid)
		row, ok := rowsMap[key]
		if !ok {
			return &mockSQLRows{cols: []string{"val"}, data: nil}, nil
		}
		// Return state or data or 1
		if s.query[7:12] == "state" {
			return &mockSQLRows{cols: []string{"state"}, data: [][]driver.Value{{row.state}}}, nil
		}
		if s.query[7:11] == "data" {
			return &mockSQLRows{cols: []string{"data"}, data: [][]driver.Value{{row.data}}}, nil
		}
		return &mockSQLRows{cols: []string{"1"}, data: [][]driver.Value{{int64(1)}}}, nil
	}

	return &mockSQLRows{cols: []string{"1"}, data: nil}, nil
}

type mockSQLRows struct {
	cols []string
	data [][]driver.Value
	idx  int
}

func (r *mockSQLRows) Columns() []string { return r.cols }
func (r *mockSQLRows) Close() error      { return nil }
func (r *mockSQLRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.data) {
		return io.EOF
	}
	for i, v := range r.data[r.idx] {
		dest[i] = v
	}
	r.idx++
	return nil
}

func TestSQLStorageOperations(t *testing.T) {
	db, err := sql.Open("fsm_mock", "test.db")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}

	storage, err := NewSQLStorage(db)
	if err != nil {
		t.Fatalf("failed to create SQLStorage: %v", err)
	}

	ctx := context.Background()
	key := SessionKey{UserID: 42, ChatID: 100}

	// 1. Initial state should be empty
	st, err := storage.GetState(ctx, key)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}
	if st != StateEmpty {
		t.Fatalf("expected empty state, got %v", st)
	}

	// 2. Set State
	if err := storage.SetState(ctx, key, "AWAITING_NAME"); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}

	st, err = storage.GetState(ctx, key)
	if err != nil || st != "AWAITING_NAME" {
		t.Fatalf("expected AWAITING_NAME, got %v (err: %v)", st, err)
	}

	// 3. Set Data
	data := map[string]any{"age": float64(25), "role": "admin"}
	if err := storage.SetData(ctx, key, data); err != nil {
		t.Fatalf("SetData failed: %v", err)
	}

	retrieved, err := storage.GetData(ctx, key)
	if err != nil {
		t.Fatalf("GetData failed: %v", err)
	}
	if retrieved["role"] != "admin" || retrieved["age"] != float64(25) {
		t.Fatalf("GetData returned unexpected values: %v", retrieved)
	}

	// 4. Reset
	if err := storage.Reset(ctx, key); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	st, _ = storage.GetState(ctx, key)
	if st != StateEmpty {
		t.Fatalf("expected StateEmpty after reset, got %v", st)
	}
}
