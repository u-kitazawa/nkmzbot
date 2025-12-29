package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

// RunMigrations runs database migrations
func (db *DB) RunMigrations(ctx context.Context) error {
	// Load and execute all .sql files under ./migrations in lexical order
	migrationsDir := "./migrations"

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".sql") {
			files = append(files, filepath.Join(migrationsDir, name))
		}
	}

	sort.Strings(files)

	for _, file := range files {
		contentBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		sqlText := strings.TrimSpace(string(contentBytes))
		if sqlText == "" {
			continue
		}

		tx, err := db.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin tx for %s: %w", file, err)
		}
		if _, execErr := tx.Exec(ctx, sqlText); execErr != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to execute migration %s: %w", file, execErr)
		}
		if err := tx.Commit(ctx); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to commit migration %s: %w", file, err)
		}
	}

	return nil
}
