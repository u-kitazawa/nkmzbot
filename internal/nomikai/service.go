package nomikai

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/susu3304/nkmzbot/internal/db"
)

type Service struct {
	mu sync.Mutex
	db *db.DB
}

func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

func (s *Service) StartSession(ctx context.Context, channelID string, guildID int64, organizerID string, roundingUnit int, remainderStrategy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if channelID == "" || guildID == 0 || organizerID == "" {
		return errors.New("必要な情報が不足しています")
	}
	if _, err := s.db.ActiveEventByChannel(ctx, channelID); err == nil {
		// already active; do nothing
		return nil
	}
	_, err := s.db.CreateEvent(ctx, guildID, channelID, organizerID, roundingUnit, remainderStrategy)
	return err
}

func (s *Service) StopSession(ctx context.Context, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, err := s.db.ActiveEventByChannel(ctx, channelID)
	if err != nil {
		return errors.New("セッションが存在しません")
	}
	return s.db.CloseEvent(ctx, ev.ID)
}

func (s *Service) Join(ctx context.Context, channelID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, err := s.db.ActiveEventByChannel(ctx, channelID)
	if err != nil {
		return errors.New("セッションが開始されていません")
	}
	return s.db.UpsertMember(ctx, ev.ID, userID, 1.0)
}

func (s *Service) SetWeight(ctx context.Context, channelID, userID string, w float64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, err := s.db.ActiveEventByChannel(ctx, channelID)
	if err != nil {
		return false, errors.New("セッションが開始されていません")
	}
	// detect existing
	members, err := s.db.Members(ctx, ev.ID)
	if err != nil {
		return false, err
	}
	joined := true
	for _, m := range members {
		if m.UserID == userID {
			joined = false
			break
		}
	}
	if w <= 0 {
		w = 0
	}
	return joined, s.db.UpsertMember(ctx, ev.ID, userID, w)
}

func (s *Service) AddPayment(ctx context.Context, channelID, userID string, amount int64, memo string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, err := s.db.ActiveEventByChannel(ctx, channelID)
	if err != nil {
		return false, errors.New("セッションが開始されていません")
	}
	// auto-join if not exists
	members, err := s.db.Members(ctx, ev.ID)
	if err != nil {
		return false, err
	}
	joined := true
	for _, m := range members {
		if m.UserID == userID {
			joined = false
			break
		}
	}
	if joined {
		if err := s.db.UpsertMember(ctx, ev.ID, userID, 1.0); err != nil {
			return false, err
		}
	}
	if amount == 0 {
		return joined, nil
	}
	_, err = s.db.AddPayment(ctx, ev.ID, userID, amount, memo, nil)
	if err != nil {
		return false, err
	}
	return joined, nil
}

// AddPaymentFor records a payment by payer for specific beneficiaries. If beneficiaries is empty, use AddPayment instead.
// Returns: payerJoined, beneficiariesJoinedIDs, error
func (s *Service) AddPaymentFor(ctx context.Context, channelID, payerID string, amount int64, memo string, beneficiaries []string) (bool, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, err := s.db.ActiveEventByChannel(ctx, channelID)
	if err != nil {
		return false, nil, errors.New("セッションが開始されていません")
	}
	members, err := s.db.Members(ctx, ev.ID)
	if err != nil {
		return false, nil, err
	}
	joined := true
	for _, m := range members {
		if m.UserID == payerID {
			joined = false
			break
		}
	}
	if joined {
		if err := s.db.UpsertMember(ctx, ev.ID, payerID, 1.0); err != nil {
			return false, nil, err
		}
	}
	// normalize beneficiaries and ensure membership
	uniq := make(map[string]struct{}, len(beneficiaries))
	var ben []string
	var benJoined []string
	for _, id := range beneficiaries {
		if id == "" {
			continue
		}
		if _, seen := uniq[id]; seen {
			continue
		}
		uniq[id] = struct{}{}
		ben = append(ben, id)
		present := false
		for _, m := range members {
			if m.UserID == id {
				present = true
				break
			}
		}
		if !present {
			if err := s.db.UpsertMember(ctx, ev.ID, id, 1.0); err != nil {
				return false, nil, err
			}
			benJoined = append(benJoined, id)
		}
	}
	if amount == 0 {
		return joined, benJoined, nil
	}
	if _, err := s.db.AddPayment(ctx, ev.ID, payerID, amount, memo, ben); err != nil {
		return false, nil, err
	}
	return joined, benJoined, nil
}

func (s *Service) Status(ctx context.Context, channelID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, err := s.db.ActiveEventByChannel(ctx, channelID)
	if err != nil {
		return "セッションが開始されていません", nil
	}
	members, err := s.db.Members(ctx, ev.ID)
	if err != nil {
		return "エラー: 参加者取得に失敗", err
	}
	if len(members) == 0 {
		return "参加者がいません", nil
	}
	pays, err := s.db.Payments(ctx, ev.ID)
	if err != nil {
		return "エラー: 支払い取得に失敗", err
	}
	paidSum := make(map[string]int64)
	for _, p := range pays {
		paidSum[p.PayerID] += p.Amount
	}
	sort.Slice(members, func(i, j int) bool { return members[i].UserID < members[j].UserID })
	var total int64
	for _, p := range pays {
		total += p.Amount
	}
	var b strings.Builder
	fmt.Fprintf(&b, "総支出: %d 円\n", total)
	for _, m := range members {
		fmt.Fprintf(&b, "<@%s> weight=%.2f paid=%d\n", m.UserID, m.Weight, paidSum[m.UserID])
	}

	// Show current pending settlement tasks (reflects /nomikai seisan).
	tasks, err := s.db.ListPendingSettlementTasks(ctx, ev.ID)
	if err != nil {
		return "エラー: 未払いタスク取得に失敗", err
	}
	if len(tasks) > 0 {
		b.WriteString("\n未払いタスク:\n")
		for _, t := range tasks {
			fmt.Fprintf(&b, "<@%s> → <@%s>: %d 円\n", t.PayerID, t.PayeeID, t.Amount)
		}
	}
	return b.String(), nil
}

// Members returns the list of participant user IDs for the channel session.
func (s *Service) Members(ctx context.Context, channelID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, err := s.db.ActiveEventByChannel(ctx, channelID)
	if err != nil {
		return nil, errors.New("セッションが開始されていません")
	}
	members, err := s.db.Members(ctx, ev.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.UserID)
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *Service) Settle(ctx context.Context, channelID string) (*SettleResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, err := s.db.ActiveEventByChannel(ctx, channelID)
	if err != nil {
		return nil, errors.New("セッションが開始されていません")
	}
	members, err := s.db.Members(ctx, ev.ID)
	if err != nil {
		return nil, err
	}
	if len(members) < 2 {
		return nil, errors.New("参加者が2人以上必要です")
	}
	pays, err := s.db.Payments(ctx, ev.ID)
	if err != nil {
		return nil, err
	}
	// Build map of weights
	weights := make(map[string]float64, len(members))
	for _, m := range members {
		weights[m.UserID] = m.Weight
	}
	// Compute charges by beneficiaries
	type bal struct {
		uid string
		net float64
	}
	charges := make(map[string]float64, len(members))
	paidSum := make(map[string]float64, len(members))
	for _, p := range pays {
		paidSum[p.PayerID] += float64(p.Amount)
	}
	for _, pay := range pays {
		// beneficiaries
		ben, err := s.db.PaymentBeneficiaries(ctx, pay.ID)
		if err != nil {
			return nil, err
		}
		var targets []string
		if len(ben) > 0 {
			targets = ben
		} else {
			targets = make([]string, 0, len(members))
			for _, m := range members {
				targets = append(targets, m.UserID)
			}
		}
		var wsum float64
		for _, uid := range targets {
			wsum += weights[uid]
		}
		if wsum == 0 {
			continue
		}
		for _, uid := range targets {
			share := float64(pay.Amount) * (weights[uid] / wsum)
			charges[uid] += share
		}
	}
	// Base balance per user: positive means they should receive, negative means they should pay.
	balance := make(map[string]float64, len(weights))
	for uid := range weights {
		balance[uid] = paidSum[uid] - charges[uid]
	}

	// Apply already-registered settlement payments (seisan) to balances.
	// If A pays B, A's balance increases (less debt), B's balance decreases (less receivable).
	paidRows, err := s.db.ListSettlementPaymentsSum(ctx, ev.ID)
	if err != nil {
		return nil, err
	}
	for _, pr := range paidRows {
		if _, ok := balance[pr.PayerID]; ok {
			balance[pr.PayerID] += float64(pr.Amount)
		}
		if _, ok := balance[pr.PayeeID]; ok {
			balance[pr.PayeeID] -= float64(pr.Amount)
		}
	}

	var pos, neg []bal
	for uid, net := range balance {
		if net > 0 {
			pos = append(pos, bal{uid: uid, net: net})
		} else if net < 0 {
			neg = append(neg, bal{uid: uid, net: -net})
		}
	}
	sort.Slice(pos, func(i, j int) bool { return pos[i].net > pos[j].net })
	sort.Slice(neg, func(i, j int) bool { return neg[i].net > neg[j].net })
	var tasks []SettlementTask
	var rows []db.SettlementTaskRow
	i, j := 0, 0
	for i < len(pos) && j < len(neg) {
		c := pos[i]
		d := neg[j]
		amt := math.Min(c.net, d.net)
		ia := int64(math.Round(amt))
		if ia > 0 {
			tasks = append(tasks, SettlementTask{PayerID: d.uid, PayeeID: c.uid, Amount: ia})
			rows = append(rows, db.SettlementTaskRow{PayerID: d.uid, PayeeID: c.uid, Amount: ia})
		}
		c.net -= amt
		d.net -= amt
		if c.net <= 1e-9 {
			i++
		} else {
			pos[i] = c
		}
		if d.net <= 1e-9 {
			j++
		} else {
			neg[j] = d
		}
	}
	// Persist tasks
	if err := s.db.SetSettlementTasks(ctx, ev.ID, rows); err != nil {
		return nil, err
	}
	var b strings.Builder
	if len(tasks) == 0 {
		b.WriteString("精算は不要です")
	} else {
		b.WriteString("支払タスク:\n")
		for _, t := range tasks {
			fmt.Fprintf(&b, "<@%s> → <@%s>: %d 円\n", t.PayerID, t.PayeeID, t.Amount)
		}
	}
	return &SettleResult{Tasks: tasks, Summary: b.String()}, nil
}

// ConfigureReminder enables or disables periodic reminders and schedules the next run.
func (s *Service) ConfigureReminder(ctx context.Context, channelID string, intervalMinutes int, disable bool, sendNow bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ev, err := s.db.ActiveEventByChannel(ctx, channelID)
	if err != nil {
		return "セッションが開始されていません", nil
	}
	if intervalMinutes <= 0 {
		cfg, err := s.db.ReminderConfig(ctx, ev.ID)
		if err != nil {
			return "リマインド設定の取得に失敗しました", err
		}
		if cfg != nil && cfg.IntervalMinutes > 0 {
			intervalMinutes = cfg.IntervalMinutes
		} else {
			intervalMinutes = 1440
		}
	}
	if intervalMinutes < 1 {
		intervalMinutes = 1
	}

	if disable {
		if err := s.db.UpsertReminder(ctx, ev.ID, false, intervalMinutes, nil); err != nil {
			return "リマインド設定の更新に失敗しました", err
		}
		return "リマインドを停止しました", nil
	}

	if sendNow {
		msg, err := s.reminderMessage(ctx, ev.ID)
		if err != nil {
			return "リマインド本文の生成に失敗しました", err
		}
		// 未払いがない場合は「次回送信」を案内すると違和感があるので、
		// next_due_at を NULL にして「未払いがある時だけ送信」モードにする。
		if msg == "未払いのタスクはありません" {
			if err := s.db.UpsertReminder(ctx, ev.ID, true, intervalMinutes, nil); err != nil {
				return "リマインド設定の更新に失敗しました", err
			}
			return msg + "\nリマインドは有効です（未払いがあるときのみ自動送信します）", nil
		}

		next := time.Now().Add(time.Duration(intervalMinutes) * time.Minute)
		if err := s.db.UpsertReminder(ctx, ev.ID, true, intervalMinutes, &next); err != nil {
			return "リマインド設定の更新に失敗しました", err
		}
		return msg + fmt.Sprintf("\n次回は約 %d 分後に自動送信します", intervalMinutes), nil
	}

	next := time.Now().Add(time.Duration(intervalMinutes) * time.Minute)
	if err := s.db.UpsertReminder(ctx, ev.ID, true, intervalMinutes, &next); err != nil {
		return "リマインド設定の更新に失敗しました", err
	}
	return fmt.Sprintf("リマインドを有効化しました。次回は約 %d 分後に送信します", intervalMinutes), nil
}

// ReminderMessage creates the current unpaid summary for a channel's event.
func (s *Service) ReminderMessage(ctx context.Context, channelID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ev, err := s.db.ActiveEventByChannel(ctx, channelID)
	if err != nil {
		return "セッションが開始されていません", nil
	}
	return s.reminderMessage(ctx, ev.ID)
}

// ReminderMessageByEventID builds a reminder body for a known event.
func (s *Service) ReminderMessageByEventID(ctx context.Context, eventID int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reminderMessage(ctx, eventID)
}

// RegisterPayment records a settlement payment between payer and payee and updates tasks.
// If payAll is true, amount is ignored and the full outstanding amount is used.
func (s *Service) RegisterPayment(ctx context.Context, channelID, payerID, payeeID string, amount int64, memo string, actorID string, payAll bool) (string, error) {
	if !payAll && amount <= 0 {
		return "金額は正の値で指定してください", nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ev, err := s.db.ActiveEventByChannel(ctx, channelID)
	if err != nil {
		return "セッションが開始されていません", nil
	}

	if payAll {
		paidAmount, err := s.db.RecordSettlementPaymentAll(ctx, ev.ID, payerID, payeeID, memo, actorID)
		if err != nil {
			return "支払いの登録に失敗しました", err
		}
		if paidAmount <= 0 {
			return "未払いタスクがありません（先に /nomikai settle を実行してください）", nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "支払いを記録しました: <@%s> → <@%s> 未払い全額 %d 円", payerID, payeeID, paidAmount)
		if memo != "" {
			fmt.Fprintf(&b, " (%s)", memo)
		}
		b.WriteString("\nこのペアの未払いタスクは解消されました")
		return b.String(), nil
	}

	remaining, err := s.db.RecordSettlementPayment(ctx, ev.ID, payerID, payeeID, amount, memo, actorID)
	if err != nil {
		return "支払いの登録に失敗しました", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "支払いを記録しました: <@%s> → <@%s> %d 円", payerID, payeeID, amount)
	if memo != "" {
		fmt.Fprintf(&b, " (%s)", memo)
	}
	if remaining > 0 {
		fmt.Fprintf(&b, "\nこのペアの未払い残高: %d 円", remaining)
	} else {
		b.WriteString("\nこのペアの未払いタスクは解消されました")
	}
	return b.String(), nil
}

func (s *Service) reminderMessage(ctx context.Context, eventID int64) (string, error) {
	tasks, err := s.db.ListPendingSettlementTasks(ctx, eventID)
	if err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		return "未払いのタスクはありません", nil
	}

	var b strings.Builder
	b.WriteString("未払いのリマインドです。対応をお願いします。\n")
	for _, t := range tasks {
		fmt.Fprintf(&b, "<@%s> → <@%s>: %d 円\n", t.PayerID, t.PayeeID, t.Amount)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
