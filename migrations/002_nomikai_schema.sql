-- Nomikai (warikan) schema

-- Events
CREATE TABLE IF NOT EXISTS nomikai_events (
    id BIGSERIAL PRIMARY KEY,
    guild_id BIGINT NOT NULL,
    channel_id TEXT NOT NULL,
    organizer_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    rounding_unit INTEGER NOT NULL DEFAULT 1,
    remainder_strategy TEXT NOT NULL DEFAULT 'organizer',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP NULL
);
CREATE INDEX IF NOT EXISTS idx_nomikai_events_guild ON nomikai_events(guild_id);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_nomikai_events_active_channel ON nomikai_events(channel_id) WHERE status = 'active';

-- Event Members
CREATE TABLE IF NOT EXISTS nomikai_event_members (
    event_id BIGINT NOT NULL REFERENCES nomikai_events(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    weight DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    PRIMARY KEY (event_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_nomikai_members_event ON nomikai_event_members(event_id);

-- Items (optional, for future detailed item-based split)
CREATE TABLE IF NOT EXISTS nomikai_items (
    id BIGSERIAL PRIMARY KEY,
    event_id BIGINT NOT NULL REFERENCES nomikai_events(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    price BIGINT NOT NULL,
    qty INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_nomikai_items_event ON nomikai_items(event_id);

-- Item exclusions (who is excluded from an item)
CREATE TABLE IF NOT EXISTS nomikai_item_exclusions (
    item_id BIGINT NOT NULL REFERENCES nomikai_items(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    PRIMARY KEY (item_id, user_id)
);

-- Payments
CREATE TABLE IF NOT EXISTS nomikai_payments (
    id BIGSERIAL PRIMARY KEY,
    event_id BIGINT NOT NULL REFERENCES nomikai_events(id) ON DELETE CASCADE,
    payer_id TEXT NOT NULL,
    amount BIGINT NOT NULL,
    memo TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_nomikai_payments_event ON nomikai_payments(event_id);
CREATE INDEX IF NOT EXISTS idx_nomikai_payments_payer ON nomikai_payments(payer_id);

-- Payment beneficiaries (who benefited from a payment)
CREATE TABLE IF NOT EXISTS nomikai_payment_beneficiaries (
    payment_id BIGINT NOT NULL REFERENCES nomikai_payments(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    PRIMARY KEY (payment_id, user_id)
);

-- Settlement tasks (computed results to track completion)
CREATE TABLE IF NOT EXISTS nomikai_settlement_tasks (
    id BIGSERIAL PRIMARY KEY,
    event_id BIGINT NOT NULL REFERENCES nomikai_events(id) ON DELETE CASCADE,
    payer_id TEXT NOT NULL,
    payee_id TEXT NOT NULL,
    amount BIGINT NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMP NULL
);
CREATE INDEX IF NOT EXISTS idx_nomikai_tasks_event ON nomikai_settlement_tasks(event_id);
CREATE INDEX IF NOT EXISTS idx_nomikai_tasks_pair ON nomikai_settlement_tasks(payer_id, payee_id);

-- Reminders
CREATE TABLE IF NOT EXISTS nomikai_reminders (
    event_id BIGINT PRIMARY KEY REFERENCES nomikai_events(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    interval_minutes INTEGER NOT NULL DEFAULT 1440,
    next_due_at TIMESTAMP NULL,
    last_sent_at TIMESTAMP NULL
);

-- Cross-session debts ledger
CREATE TABLE IF NOT EXISTS nomikai_debts (
    id BIGSERIAL PRIMARY KEY,
    guild_id BIGINT NOT NULL,
    lender_id TEXT NOT NULL,
    borrower_id TEXT NOT NULL,
    amount BIGINT NOT NULL,
    origin_event_id BIGINT NULL REFERENCES nomikai_events(id) ON DELETE SET NULL,
    note TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    settled_at TIMESTAMP NULL
);
CREATE INDEX IF NOT EXISTS idx_nomikai_debts_guild ON nomikai_debts(guild_id);
CREATE INDEX IF NOT EXISTS idx_nomikai_debts_pair ON nomikai_debts(lender_id, borrower_id);
