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
	tables := []string{
		"channels", "orders", "reactions", "polls", "giveaways", "media_history",
		"users", "user_identities", "user_mode_states", "mode_configs", "leads",
		"mascot_phrases", "products", "services", "time_slots", "bookings",
		"events", "event_registrations", "onboarding_submissions", "knowledge_documents",
		"knowledge_chunks", "support_tickets", "support_messages", "chat_sessions",
		"admin_audit_logs", "image_projects", "image_revisions",
	}
	for _, tbl := range tables {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?;"
		err := db.QueryRow(query, tbl).Scan(&name)
		if err != nil {
			t.Errorf("expected table %q to exist in sqlite schema, got error: %v", tbl, err)
		}
	}

	var modeTitle string
	if err := db.QueryRow("SELECT title FROM mode_configs WHERE mode_id='images';").Scan(&modeTitle); err != nil {
		t.Errorf("expected mode_configs row for 'images' to exist, got error: %v", err)
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
