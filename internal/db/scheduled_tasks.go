package db

import (
	"context"
	"time"
)

type ScheduledTask struct {
	ID        int       `json:"id"`
	Command   string    `json:"command"`
	Time      time.Time `json:"time"`
	Repeat    bool      `json:"repeat"`
	ChannelID string    `json:"channel_id"`
	GuildID   int64     `json:"guild_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (db *DB) AddScheduledTask(ctx context.Context, command string, execTime time.Time, repeat bool, channelID string, guildID int64, userID string) (*ScheduledTask, error) {
	var task ScheduledTask
	err := db.pool.QueryRow(ctx,
		`INSERT INTO scheduled_tasks (command, time, repeat, channel_id, guild_id, user_id) 
		VALUES ($1, $2, $3, $4, $5, $6) 
		RETURNING id, command, time, repeat, channel_id, guild_id, user_id, created_at`,
		command, execTime, repeat, channelID, guildID, userID,
	).Scan(&task.ID, &task.Command, &task.Time, &task.Repeat, &task.ChannelID, &task.GuildID, &task.UserID, &task.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (db *DB) GetScheduledTask(ctx context.Context, id int) (*ScheduledTask, error) {
	var task ScheduledTask
	err := db.pool.QueryRow(ctx,
		`SELECT id, command, time, repeat, channel_id, guild_id, user_id, created_at 
		FROM scheduled_tasks WHERE id = $1`,
		id,
	).Scan(&task.ID, &task.Command, &task.Time, &task.Repeat, &task.ChannelID, &task.GuildID, &task.UserID, &task.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (db *DB) ListScheduledTasks(ctx context.Context, guildID int64) ([]*ScheduledTask, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, command, time, repeat, channel_id, guild_id, user_id, created_at 
		FROM scheduled_tasks WHERE guild_id = $1 ORDER BY time`,
		guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*ScheduledTask
	for rows.Next() {
		var task ScheduledTask
		if err := rows.Scan(&task.ID, &task.Command, &task.Time, &task.Repeat, &task.ChannelID, &task.GuildID, &task.UserID, &task.CreatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (db *DB) ListAllScheduledTasks(ctx context.Context) ([]*ScheduledTask, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, command, time, repeat, channel_id, guild_id, user_id, created_at 
		FROM scheduled_tasks ORDER BY time`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*ScheduledTask
	for rows.Next() {
		var task ScheduledTask
		if err := rows.Scan(&task.ID, &task.Command, &task.Time, &task.Repeat, &task.ChannelID, &task.GuildID, &task.UserID, &task.CreatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (db *DB) UpdateScheduledTaskTime(ctx context.Context, id int, newTime time.Time) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE scheduled_tasks SET time = $2 WHERE id = $1`,
		id, newTime,
	)
	return err
}

func (db *DB) DeleteScheduledTask(ctx context.Context, id int) error {
	_, err := db.pool.Exec(ctx,
		`DELETE FROM scheduled_tasks WHERE id = $1`,
		id,
	)
	return err
}
