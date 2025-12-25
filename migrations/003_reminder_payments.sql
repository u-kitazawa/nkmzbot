-- Settlement payment logs (actual transfers between users)
CREATE TABLE IF NOT EXISTS nomikai_task_payments (
    id BIGSERIAL PRIMARY KEY,
    event_id BIGINT NOT NULL REFERENCES nomikai_events(id) ON DELETE CASCADE,
    payer_id TEXT NOT NULL,
    payee_id TEXT NOT NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    memo TEXT,
    recorded_by TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_nomikai_task_payments_event ON nomikai_task_payments(event_id);
CREATE INDEX IF NOT EXISTS idx_nomikai_task_payments_pair ON nomikai_task_payments(payer_id, payee_id);

-- Ensure reminder schedule columns remain nullable for scheduling
ALTER TABLE nomikai_reminders
    ALTER COLUMN next_due_at DROP NOT NULL,
    ALTER COLUMN last_sent_at DROP NOT NULL;
