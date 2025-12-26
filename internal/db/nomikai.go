package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type ReminderConfig struct {
	Enabled         bool
	IntervalMinutes int
	NextDueAt       *time.Time
}

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

// UpsertReminder configures reminders for an event and optionally schedules the next due time.
func (db *DB) UpsertReminder(ctx context.Context, eventID int64, enabled bool, intervalMinutes int, nextDueAt *time.Time) error {
	_, err := db.pool.Exec(ctx,
		`INSERT INTO nomikai_reminders (event_id, enabled, interval_minutes, next_due_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (event_id) DO UPDATE
		 SET enabled = EXCLUDED.enabled,
			 interval_minutes = EXCLUDED.interval_minutes,
			 next_due_at = COALESCE(EXCLUDED.next_due_at, nomikai_reminders.next_due_at)`,
		eventID, enabled, intervalMinutes, nextDueAt,
	)
	return err
}

func (db *DB) ReminderConfig(ctx context.Context, eventID int64) (*ReminderConfig, error) {
	row := db.pool.QueryRow(ctx,
		`SELECT enabled, interval_minutes, next_due_at
		 FROM nomikai_reminders
		 WHERE event_id = $1`,
		eventID,
	)
	var cfg ReminderConfig
	var nextDueAt *time.Time
	if err := row.Scan(&cfg.Enabled, &cfg.IntervalMinutes, &nextDueAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	cfg.NextDueAt = nextDueAt
	return &cfg, nil
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

type ReminderDue struct {
	EventID         int64
	ChannelID       string
	IntervalMinutes int
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

// ListPendingSettlementTasks returns unsettled tasks for an event.
func (db *DB) ListPendingSettlementTasks(ctx context.Context, eventID int64) ([]SettlementTaskRow, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT payer_id, payee_id, amount
		 FROM nomikai_settlement_tasks
		 WHERE event_id = $1 AND completed = FALSE
		 ORDER BY payer_id, payee_id`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []SettlementTaskRow
	for rows.Next() {
		var t SettlementTaskRow
		if err := rows.Scan(&t.PayerID, &t.PayeeID, &t.Amount); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ListSettlementPaymentsSum returns total settlement payments per payer/payee pair for an event.
func (db *DB) ListSettlementPaymentsSum(ctx context.Context, eventID int64) ([]SettlementTaskRow, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT payer_id, payee_id, COALESCE(SUM(amount), 0)
		 FROM nomikai_task_payments
		 WHERE event_id = $1
		 GROUP BY payer_id, payee_id
		 ORDER BY payer_id, payee_id`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SettlementTaskRow
	for rows.Next() {
		var r SettlementTaskRow
		if err := rows.Scan(&r.PayerID, &r.PayeeID, &r.Amount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DueReminders returns reminder targets that are due and still have pending tasks.
func (db *DB) DueReminders(ctx context.Context, now time.Time) ([]ReminderDue, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT r.event_id, e.channel_id, r.interval_minutes
		 FROM nomikai_reminders r
		 JOIN nomikai_events e ON e.id = r.event_id
		 WHERE r.enabled = TRUE
		   AND (r.next_due_at IS NULL OR r.next_due_at <= $1)
		   AND EXISTS (
			 SELECT 1 FROM nomikai_settlement_tasks t
			 WHERE t.event_id = r.event_id AND t.completed = FALSE
		   )`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []ReminderDue
	for rows.Next() {
		var r ReminderDue
		if err := rows.Scan(&r.EventID, &r.ChannelID, &r.IntervalMinutes); err != nil {
			return nil, err
		}
		targets = append(targets, r)
	}
	return targets, rows.Err()
}

// MarkReminderSent updates reminder schedule timestamps.
func (db *DB) MarkReminderSent(ctx context.Context, eventID int64, sentAt time.Time, nextDue time.Time) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE nomikai_reminders
		 SET last_sent_at = $2, next_due_at = $3
		 WHERE event_id = $1`,
		eventID, sentAt, nextDue,
	)
	return err
}

// DelayReminder updates next_due_at without touching last_sent_at.
func (db *DB) DelayReminder(ctx context.Context, eventID int64, nextDue time.Time) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE nomikai_reminders
		 SET next_due_at = $2
		 WHERE event_id = $1`,
		eventID, nextDue,
	)
	return err
}

// RecordSettlementPayment logs a settlement payment and reduces outstanding tasks (payer -> payee).
// Returns the remaining unsettled amount for the pair after applying the payment.
func (db *DB) RecordSettlementPayment(ctx context.Context, eventID int64, payerID, payeeID string, amount int64, memo, recordedBy string) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	type pending struct {
		ID     int64
		Amount int64
	}

	rows, err := tx.Query(ctx,
		`SELECT id, amount
		 FROM nomikai_settlement_tasks
		 WHERE event_id = $1 AND completed = FALSE AND payer_id = $2 AND payee_id = $3
		 ORDER BY id FOR UPDATE`,
		eventID, payerID, payeeID,
	)
	if err != nil {
		return 0, err
	}
	var tasks []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.ID, &p.Amount); err != nil {
			rows.Close()
			return 0, err
		}
		tasks = append(tasks, p)
	}
	rows.Close()

	remainingPayment := amount
	for _, t := range tasks {
		if remainingPayment <= 0 {
			break
		}
		switch {
		case remainingPayment >= t.Amount:
			remainingPayment -= t.Amount
			if _, err := tx.Exec(ctx,
				`UPDATE nomikai_settlement_tasks
				 SET completed = TRUE, completed_at = COALESCE(completed_at, CURRENT_TIMESTAMP)
				 WHERE id = $1`,
				t.ID,
			); err != nil {
				return 0, err
			}
		default:
			newAmount := t.Amount - remainingPayment
			remainingPayment = 0
			if _, err := tx.Exec(ctx,
				`UPDATE nomikai_settlement_tasks SET amount = $2 WHERE id = $1`,
				t.ID, newAmount,
			); err != nil {
				return 0, err
			}
		}
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO nomikai_task_payments (event_id, payer_id, payee_id, amount, memo, recorded_by)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		eventID, payerID, payeeID, amount, memo, recordedBy,
	); err != nil {
		return 0, err
	}

	var remaining int64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0)
		 FROM nomikai_settlement_tasks
		 WHERE event_id = $1 AND completed = FALSE AND payer_id = $2 AND payee_id = $3`,
		eventID, payerID, payeeID,
	).Scan(&remaining); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return remaining, nil
}
