package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Command struct {
	GuildID  int64  `json:"guild_id"`
	Name     string `json:"name"`
	Response string `json:"response"`
}

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

func (db *DB) GetCommand(ctx context.Context, guildID int64, name string) (*Command, error) {
	var cmd Command
	err := db.pool.QueryRow(ctx,
		"SELECT guild_id, name, response FROM commands WHERE guild_id = $1 AND name = $2",
		guildID, name,
	).Scan(&cmd.GuildID, &cmd.Name, &cmd.Response)
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (db *DB) AddCommand(ctx context.Context, guildID int64, name, response string) error {
	_, err := db.pool.Exec(ctx,
		"INSERT INTO commands (guild_id, name, response) VALUES ($1, $2, $3) ON CONFLICT (guild_id, name) DO NOTHING",
		guildID, name, response,
	)
	return err
}

func (db *DB) UpdateCommand(ctx context.Context, guildID int64, name, response string) error {
	result, err := db.pool.Exec(ctx,
		"UPDATE commands SET response = $3 WHERE guild_id = $1 AND name = $2",
		guildID, name, response,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("command not found")
	}
	return nil
}

func (db *DB) RemoveCommand(ctx context.Context, guildID int64, name string) error {
	result, err := db.pool.Exec(ctx,
		"DELETE FROM commands WHERE guild_id = $1 AND name = $2",
		guildID, name,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("command not found")
	}
	return nil
}

func (db *DB) ListCommands(ctx context.Context, guildID int64, pattern string) ([]Command, error) {
	var commands []Command
	var rows interface {
		Next() bool
		Scan(dest ...interface{}) error
		Close()
		Err() error
	}
	var err error

	if pattern != "" {
		likePattern := "%" + pattern + "%"
		rows, err = db.pool.Query(ctx,
			"SELECT guild_id, name, response FROM commands WHERE guild_id = $1 AND (name ILIKE $2 OR response ILIKE $2) ORDER BY name",
			guildID, likePattern,
		)
	} else {
		rows, err = db.pool.Query(ctx,
			"SELECT guild_id, name, response FROM commands WHERE guild_id = $1 ORDER BY name",
			guildID,
		)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cmd Command
		if err := rows.Scan(&cmd.GuildID, &cmd.Name, &cmd.Response); err != nil {
			return nil, err
		}
		commands = append(commands, cmd)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return commands, nil
}

func (db *DB) GetRegisteredGuildIDs(ctx context.Context) ([]int64, error) {
	rows, err := db.pool.Query(ctx, "SELECT DISTINCT guild_id FROM commands")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var guildIDs []int64
	for rows.Next() {
		var guildID int64
		if err := rows.Scan(&guildID); err != nil {
			return nil, err
		}
		guildIDs = append(guildIDs, guildID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return guildIDs, nil
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
