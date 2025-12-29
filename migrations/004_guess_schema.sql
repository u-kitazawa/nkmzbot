-- Guess (GeoGuessr) schema

-- Sessions
CREATE TABLE IF NOT EXISTS guess_sessions (
    id BIGSERIAL PRIMARY KEY,
    channel_id TEXT NOT NULL,
    guild_id BIGINT NOT NULL,
    organizer_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    answer_lat DOUBLE PRECISION NULL,
    answer_lng DOUBLE PRECISION NULL,
    answer_url TEXT NULL,
    max_error_distance DOUBLE PRECISION NOT NULL DEFAULT 20015086.796,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_guess_sessions_active_channel ON guess_sessions(channel_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_guess_sessions_guild ON guess_sessions(guild_id);

-- Guesses
CREATE TABLE IF NOT EXISTS guess_guesses (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL REFERENCES guess_sessions(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    guess_lat DOUBLE PRECISION NOT NULL,
    guess_lng DOUBLE PRECISION NOT NULL,
    guess_url TEXT NOT NULL,
    score INTEGER NULL,
    distance_meters DOUBLE PRECISION NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(session_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_guess_guesses_session ON guess_guesses(session_id);
CREATE INDEX IF NOT EXISTS idx_guess_guesses_user ON guess_guesses(user_id);
