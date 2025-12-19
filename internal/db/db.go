package db

import (
	"context"
	"fmt"

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
	// Simple migration for the commands table
	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS commands (
			guild_id BIGINT NOT NULL,
			name TEXT NOT NULL,
			response TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (guild_id, name)
		);
		CREATE INDEX IF NOT EXISTS idx_commands_guild_id ON commands(guild_id);
	`)
	return err
}
