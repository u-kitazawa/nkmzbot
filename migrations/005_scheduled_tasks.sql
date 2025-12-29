-- Scheduled tasks table for jikan command
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id SERIAL PRIMARY KEY,
    command TEXT NOT NULL,
    time TIMESTAMP NOT NULL,
    repeat BOOLEAN NOT NULL DEFAULT FALSE,
    channel_id TEXT NOT NULL,
    guild_id BIGINT NOT NULL,
    user_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_guild_id ON scheduled_tasks(guild_id);
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_time ON scheduled_tasks(time);
