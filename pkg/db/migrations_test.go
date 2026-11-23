package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRunMigrations_SQLite(t *testing.T) {
	// Connect to in-memory SQLite using pure-Go CGO-free driver
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db, "sqlite3"); err != nil {
		t.Fatalf("RunMigrations failed for sqlite3: %v", err)
	}

	// Verify all tables were created
	tables := []string{"channels", "orders", "reactions", "polls", "giveaways", "media_history"}
	for _, tbl := range tables {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?;"
		err := db.QueryRow(query, tbl).Scan(&name)
		if err != nil {
			t.Errorf("expected table %q to exist in sqlite schema, got error: %v", tbl, err)
		}
	}

	// Verify idempotency: running migrations again should not fail
	if err := RunMigrations(db, "sqlite"); err != nil {
		t.Fatalf("RunMigrations failed on second run (idempotency check): %v", err)
	}
}

func TestRunMigrations_UnsupportedDialect(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db, "oracle"); err == nil {
		t.Fatalf("expected error for unsupported dialect 'oracle', got nil")
	}
}
