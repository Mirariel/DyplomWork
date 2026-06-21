package models_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/okochadmytro/tradetracker/internal/models"
)

// testDB — shared connection used by all integration tests in this package.
var testDB *sqlx.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/tradetracker_test?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci"
	}

	// Create the test database if it does not exist yet.
	if err := ensureDatabase(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "ensure test DB: %v\n", err)
		os.Exit(1)
	}

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect test DB: %v\n", err)
		os.Exit(1)
	}
	testDB = db

	if err := applyMigrations(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	_ = db.Close()
	os.Exit(code)
}

// ensureDatabase creates tradetracker_test if it doesn't exist.
func ensureDatabase(dsn string) error {
	// Connect without a database name.
	adminDSN := "root:@tcp(127.0.0.1:3306)/?parseTime=true&charset=utf8mb4"
	if v := os.Getenv("TEST_ADMIN_DSN"); v != "" {
		adminDSN = v
	}

	db, err := sql.Open("mysql", adminDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS tradetracker_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	return err
}

// applyMigrations runs all pending UP migrations on the test database.
func applyMigrations(dsn string) error {
	wd, err := os.Getwd() // internal/models/
	if err != nil {
		return err
	}
	// Go up two levels to reach project root, then into migrations/.
	migDir := filepath.ToSlash(filepath.Join(wd, "..", "..", "migrations"))
	sourceURL := "file://" + migDir

	// golang-migrate MySQL URL uses tcp() notation.
	dbURL := "mysql://" + dsn

	mig, err := migrate.New(sourceURL, dbURL)
	if err != nil {
		return fmt.Errorf("migrate.New: %w", err)
	}
	defer mig.Close()

	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// truncateAll removes all test data while respecting FK order.
// Call at the start of each test function to guarantee a clean slate.
func truncateAll(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec("SET FOREIGN_KEY_CHECKS = 0")
	if err != nil {
		t.Fatalf("disable FK checks: %v", err)
	}
	tables := []string{
		"bot_grids", "bots",
		"smart_orders", "orders",
		"dca_bots",
		"portfolio_snapshots",
		"external_api_credentials",
		"transactions",
		"user_portfolios",
		"open_positions",
		"position_history",
		"users",
	}
	for _, tbl := range tables {
		if _, err := testDB.Exec("DELETE FROM " + tbl); err != nil {
			t.Fatalf("truncate %s: %v", tbl, err)
		}
	}
	_, _ = testDB.Exec("SET FOREIGN_KEY_CHECKS = 1")
}

// createTestUser inserts a user and returns it.
func createTestUser(t *testing.T, suffix string) *models.User {
	t.Helper()
	repo := models.NewUserRepository(testDB)
	u, err := repo.Create("user_"+suffix, "user_"+suffix+"@test.com", "password123")
	if err != nil {
		t.Fatalf("createTestUser: %v", err)
	}
	return u
}
