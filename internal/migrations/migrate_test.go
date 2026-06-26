package migrations_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/joanmarcriera/labpeek-go/internal/db"
	"github.com/joanmarcriera/labpeek-go/internal/migrations"
)

func TestApplyCreatesRequiredTablesAndDatabaseFile(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "data", "labpeek.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := migrations.Apply(database); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	for _, tableName := range []string{
		"settings",
		"assets",
		"services",
		"discovery_runs",
	} {
		assertTableExists(t, database, tableName)
	}

	assertSingleStringValue(t, database, "PRAGMA journal_mode", "wal")
	assertSingleIntValue(t, database, "PRAGMA foreign_keys", 1)
	assertSingleIntValue(t, database, "PRAGMA busy_timeout", 5000)
}

func assertTableExists(t *testing.T, database *sql.DB, tableName string) {
	t.Helper()

	var name string
	if err := database.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = ? AND name = ?`,
		"table",
		tableName,
	).Scan(&name); err != nil {
		t.Fatalf("expected table %q to exist: %v", tableName, err)
	}
}

func assertSingleStringValue(t *testing.T, database *sql.DB, query string, want string) {
	t.Helper()

	var got string
	if err := database.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	if got != want {
		t.Fatalf("query %q = %q, want %q", query, got, want)
	}
}

func assertSingleIntValue(t *testing.T, database *sql.DB, query string, want int) {
	t.Helper()

	var got int
	if err := database.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	if got != want {
		t.Fatalf("query %q = %d, want %d", query, got, want)
	}
}
