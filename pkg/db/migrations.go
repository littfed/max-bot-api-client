package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/postgres/*.sql
var postgresMigrations embed.FS

//go:embed migrations/sqlite/*.sql
var sqliteMigrations embed.FS

// RunMigrations executes embedded Goose migrations for the given database connection and dialect.
// Accepted dialects: "postgres", "postgresql", "pgx", "sqlite", "sqlite3".
func RunMigrations(database *sql.DB, dialect string) error {
	return RunMigrationsContext(context.Background(), database, dialect)
}

// RunMigrationsContext executes embedded Goose migrations with context support.
// Accepted dialects: "postgres", "postgresql", "pgx", "sqlite", "sqlite3".
func RunMigrationsContext(ctx context.Context, database *sql.DB, dialect string) error {
	normalized := strings.ToLower(strings.TrimSpace(dialect))
	var subFS fs.FS
	var gooseDialect string
	var err error

	switch normalized {
	case "postgres", "postgresql", "pgx":
		gooseDialect = "postgres"
		subFS, err = fs.Sub(postgresMigrations, "migrations/postgres")
	case "sqlite", "sqlite3":
		gooseDialect = "sqlite3"
		subFS, err = fs.Sub(sqliteMigrations, "migrations/sqlite")
	default:
		return fmt.Errorf("db: unsupported database dialect %q", dialect)
	}

	if err != nil {
		return fmt.Errorf("db: failed to locate embedded migrations: %w", err)
	}

	goose.SetBaseFS(subFS)
	defer goose.SetBaseFS(nil)

	if err := goose.SetDialect(gooseDialect); err != nil {
		return fmt.Errorf("db: failed to set goose dialect %q: %w", gooseDialect, err)
	}

	if err := goose.UpContext(ctx, database, "."); err != nil {
		return fmt.Errorf("db: goose up migrations failed: %w", err)
	}

	return nil
}
