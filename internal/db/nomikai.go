package db

import (
	"context"
	"fmt"
)

type NomikaiEvent struct {
	ID                int64
	GuildID           int64
	ChannelID         string
	OrganizerID       string
	Status            string
	RoundingUnit      int
	RemainderStrategy string
}

type NomikaiMember struct {
	EventID int64
	UserID  string
	Weight  float64
}

type NomikaiPayment struct {
	ID      int64
	EventID int64
	PayerID string
	Amount  int64
	Memo    string
}

// CreateEvent creates a new active event. If there is already an active event for the channel,
// it returns an error unless allowDuplicate is true.
func (db *DB) CreateEvent(ctx context.Context, guildID int64, channelID, organizerID string, roundingUnit int, remainderStrategy string) (int64, error) {
	var id int64
	err := db.pool.QueryRow(ctx,
		`INSERT INTO nomikai_events (guild_id, channel_id, organizer_id, status, rounding_unit, remainder_strategy)
         VALUES ($1, $2, $3, 'active', $4, $5)
         RETURNING id`,
		guildID, channelID, organizerID, roundingUnit, remainderStrategy,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// CloseEvent sets the event status to closed.
func (db *DB) CloseEvent(ctx context.Context, eventID int64) error {
	ct, err := db.pool.Exec(ctx, `UPDATE nomikai_events SET status = 'closed', closed_at = CURRENT_TIMESTAMP WHERE id = $1`, eventID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("event not found")
	}
	return nil
}

// UpsertMember adds or updates a member weight.
func (db *DB) UpsertMember(ctx context.Context, eventID int64, userID string, weight float64) error {
	_, err := db.pool.Exec(ctx,
		`INSERT INTO nomikai_event_members (event_id, user_id, weight)
         VALUES ($1, $2, $3)
         ON CONFLICT (event_id, user_id) DO UPDATE SET weight = EXCLUDED.weight`,
		eventID, userID, weight,
	)
	return err
}

// AddPayment inserts a payment and optional beneficiaries.
func (db *DB) AddPayment(ctx context.Context, eventID int64, payerID string, amount int64, memo string, beneficiaries []string) (int64, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var payID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO nomikai_payments (event_id, payer_id, amount, memo)
         VALUES ($1, $2, $3, $4)
         RETURNING id`,
		eventID, payerID, amount, memo,
	).Scan(&payID); err != nil {
		return 0, err
	}

	if len(beneficiaries) > 0 {
		for _, uid := range beneficiaries {
			if uid == "" {
				continue
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO nomikai_payment_beneficiaries (payment_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				payID, uid,
			); err != nil {
				return 0, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return payID, nil
}

// Members returns all members for an event.
func (db *DB) Members(ctx context.Context, eventID int64) ([]NomikaiMember, error) {
	rows, err := db.pool.Query(ctx, `SELECT event_id, user_id, weight FROM nomikai_event_members WHERE event_id = $1`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NomikaiMember
	for rows.Next() {
		var m NomikaiMember
		if err := rows.Scan(&m.EventID, &m.UserID, &m.Weight); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Payments returns payments and does not expand beneficiaries.
func (db *DB) Payments(ctx context.Context, eventID int64) ([]NomikaiPayment, error) {
	rows, err := db.pool.Query(ctx, `SELECT id, event_id, payer_id, amount, COALESCE(memo, '') FROM nomikai_payments WHERE event_id = $1 ORDER BY id`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NomikaiPayment
	for rows.Next() {
		var p NomikaiPayment
		if err := rows.Scan(&p.ID, &p.EventID, &p.PayerID, &p.Amount, &p.Memo); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PaymentBeneficiaries returns beneficiary user IDs for a payment.
func (db *DB) PaymentBeneficiaries(ctx context.Context, paymentID int64) ([]string, error) {
	rows, err := db.pool.Query(ctx, `SELECT user_id FROM nomikai_payment_beneficiaries WHERE payment_id = $1`, paymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}

// UpsertReminder configures reminders for an event.
func (db *DB) UpsertReminder(ctx context.Context, eventID int64, enabled bool, intervalMinutes int) error {
	_, err := db.pool.Exec(ctx,
		`INSERT INTO nomikai_reminders (event_id, enabled, interval_minutes)
         VALUES ($1, $2, $3)
         ON CONFLICT (event_id) DO UPDATE SET enabled = EXCLUDED.enabled, interval_minutes = EXCLUDED.interval_minutes`,
		eventID, enabled, intervalMinutes,
	)
	return err
}

// InsertDebt records a cross-session debt.
func (db *DB) InsertDebt(ctx context.Context, guildID int64, lenderID, borrowerID string, amount int64, originEventID *int64, note string) (int64, error) {
	var id int64
	err := db.pool.QueryRow(ctx,
		`INSERT INTO nomikai_debts (guild_id, lender_id, borrower_id, amount, origin_event_id, note)
         VALUES ($1, $2, $3, $4, $5, $6)
         RETURNING id`,
		guildID, lenderID, borrowerID, amount, originEventID, note,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ActiveEventByChannel returns the active event for the given channel, if any.
func (db *DB) ActiveEventByChannel(ctx context.Context, channelID string) (*NomikaiEvent, error) {
	row := db.pool.QueryRow(ctx, `SELECT id, guild_id, channel_id, organizer_id, status, rounding_unit, remainder_strategy FROM nomikai_events WHERE channel_id = $1 AND status = 'active' LIMIT 1`, channelID)
	var ev NomikaiEvent
	if err := row.Scan(&ev.ID, &ev.GuildID, &ev.ChannelID, &ev.OrganizerID, &ev.Status, &ev.RoundingUnit, &ev.RemainderStrategy); err != nil {
		return nil, err
	}
	return &ev, nil
}

type SettlementTaskRow struct {
	PayerID string
	PayeeID string
	Amount  int64
}

// SetSettlementTasks replaces tasks for an event.
func (db *DB) SetSettlementTasks(ctx context.Context, eventID int64, tasks []SettlementTaskRow) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM nomikai_settlement_tasks WHERE event_id = $1`, eventID); err != nil {
		return err
	}
	for _, t := range tasks {
		if t.Amount <= 0 || t.PayerID == "" || t.PayeeID == "" {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO nomikai_settlement_tasks (event_id, payer_id, payee_id, amount, completed)
             VALUES ($1, $2, $3, $4, FALSE)`,
			eventID, t.PayerID, t.PayeeID, t.Amount,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// CompleteTaskPair marks a task between two users as completed.
func (db *DB) CompleteTaskPair(ctx context.Context, eventID int64, actorID, otherID string) (bool, error) {
	ct, err := db.pool.Exec(ctx,
		`UPDATE nomikai_settlement_tasks
         SET completed = TRUE, completed_at = CURRENT_TIMESTAMP
         WHERE event_id = $1 AND completed = FALSE AND (
           (payer_id = $2 AND payee_id = $3) OR (payer_id = $3 AND payee_id = $2)
         )`,
		eventID, actorID, otherID,
	)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() > 0, nil
}
