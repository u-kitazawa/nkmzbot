package nomikai

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

type Service struct {
	mu    sync.Mutex
	store map[string]*Session
}

func NewService() *Service {
	return &Service{store: make(map[string]*Session)}
}

func (s *Service) StartSession(channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.store[channelID]; ok {
		if sess.Active {
			return nil
		}
		sess.Active = true
		return nil
	}
	s.store[channelID] = &Session{
		ChannelID:    channelID,
		Active:       true,
		Participants: make(map[string]*Participant),
	}
	return nil
}

func (s *Service) StopSession(channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.store[channelID]; !ok {
		return errors.New("セッションが存在しません")
	}
	delete(s.store, channelID)
	return nil
}

func (s *Service) Join(channelID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.store[channelID]
	if !ok || !sess.Active {
		return errors.New("セッションが開始されていません")
	}
	if _, exists := sess.Participants[userID]; !exists {
		sess.Participants[userID] = &Participant{UserID: userID, Weight: 1.0}
	}
	return nil
}

func (s *Service) SetWeight(channelID, userID string, w float64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.store[channelID]
	if !ok || !sess.Active {
		return false, errors.New("セッションが開始されていません")
	}
	p, exists := sess.Participants[userID]
	if !exists {
		p = &Participant{UserID: userID, Weight: 1.0}
		sess.Participants[userID] = p
		// Newly joined
		joined := true
		if w <= 0 {
			w = 0
		}
		p.Weight = w
		return joined, nil
	}
	if w <= 0 {
		w = 0
	}
	p.Weight = w
	return false, nil
}

func (s *Service) AddPayment(channelID, userID string, amount int64, memo string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.store[channelID]
	if !ok || !sess.Active {
		return false, errors.New("セッションが開始されていません")
	}
	p, exists := sess.Participants[userID]
	if !exists {
		p = &Participant{UserID: userID, Weight: 1.0}
		sess.Participants[userID] = p
		// Newly joined
		joined := true
		if p.PaidSum+amount < 0 {
			return false, errors.New("訂正額により合計が負になります")
		}
		p.PaidSum += amount
		sess.Payments = append(sess.Payments, Payment{PayerID: userID, Amount: amount, Memo: memo})
		return joined, nil
	}
	if p.PaidSum+amount < 0 {
		return false, errors.New("訂正額により合計が負になります")
	}
	p.PaidSum += amount
	sess.Payments = append(sess.Payments, Payment{PayerID: userID, Amount: amount, Memo: memo})
	return false, nil
}

// AddPaymentFor records a payment by payer for specific beneficiaries. If beneficiaries is empty, use AddPayment instead.
// Returns: payerJoined, beneficiariesJoinedIDs, error
func (s *Service) AddPaymentFor(channelID, payerID string, amount int64, memo string, beneficiaries []string) (bool, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.store[channelID]
	if !ok || !sess.Active {
		return false, nil, errors.New("セッションが開始されていません")
	}
	// Ensure payer exists
	p, exists := sess.Participants[payerID]
	joined := false
	if !exists {
		p = &Participant{UserID: payerID, Weight: 1.0}
		sess.Participants[payerID] = p
		joined = true
	}
	if p.PaidSum+amount < 0 {
		return false, nil, errors.New("訂正額により合計が負になります")
	}
	// Ensure beneficiaries exist as participants (auto-join)
	// Also normalize: remove duplicates and empties
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
		if _, ok := sess.Participants[id]; !ok {
			sess.Participants[id] = &Participant{UserID: id, Weight: 1.0}
			benJoined = append(benJoined, id)
		}
		ben = append(ben, id)
	}
	p.PaidSum += amount
	sess.Payments = append(sess.Payments, Payment{PayerID: payerID, Amount: amount, Memo: memo, Beneficiaries: ben})
	return joined, benJoined, nil
}

func (s *Service) Status(channelID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.store[channelID]
	if !ok || !sess.Active {
		return "セッションが開始されていません", nil
	}
	if len(sess.Participants) == 0 {
		return "参加者がいません", nil
	}
	var total int64
	var wsum float64
	type row struct {
		uid  string
		w    float64
		paid int64
	}
	var rows []row
	for uid, p := range sess.Participants {
		total += p.PaidSum
		wsum += p.Weight
		rows = append(rows, row{uid: uid, w: p.Weight, paid: p.PaidSum})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].uid < rows[j].uid })
	var b strings.Builder
	fmt.Fprintf(&b, "総支出: %d 円\n", total)
	for _, r := range rows {
		fmt.Fprintf(&b, "<@%s> weight=%.2f paid=%d\n", r.uid, r.w, r.paid)
	}
	return b.String(), nil
}

// Members returns the list of participant user IDs for the channel session.
func (s *Service) Members(channelID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.store[channelID]
	if !ok || !sess.Active {
		return nil, errors.New("セッションが開始されていません")
	}
	ids := make([]string, 0, len(sess.Participants))
	for uid := range sess.Participants {
		ids = append(ids, uid)
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *Service) Settle(channelID string) (*SettleResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.store[channelID]
	if !ok || !sess.Active {
		return nil, errors.New("セッションが開始されていません")
	}
	if len(sess.Participants) < 2 {
		return nil, errors.New("参加者が2人以上必要です")
	}
	// Compute charges per user based on payment-level beneficiaries
	type bal struct {
		uid string
		net float64
	}
	charges := make(map[string]float64, len(sess.Participants))
	// Iterate payments and distribute cost among beneficiaries (or all participants if none)
	for _, pay := range sess.Payments {
		var targetIDs []string
		if len(pay.Beneficiaries) > 0 {
			targetIDs = pay.Beneficiaries
		} else {
			targetIDs = make([]string, 0, len(sess.Participants))
			for uid := range sess.Participants {
				targetIDs = append(targetIDs, uid)
			}
		}
		// sum weights of targets
		var wsum float64
		for _, uid := range targetIDs {
			if p, ok := sess.Participants[uid]; ok {
				wsum += p.Weight
			}
		}
		if wsum == 0 {
			continue // cannot allocate
		}
		for _, uid := range targetIDs {
			if p, ok := sess.Participants[uid]; ok {
				share := float64(pay.Amount) * (p.Weight / wsum)
				charges[uid] += share
			}
		}
	}
	var pos, neg []bal
	for uid, p := range sess.Participants {
		net := float64(p.PaidSum) - charges[uid]
		if net > 0 {
			pos = append(pos, bal{uid: uid, net: net})
		} else if net < 0 {
			neg = append(neg, bal{uid: uid, net: -net})
		}
	}
	sort.Slice(pos, func(i, j int) bool { return pos[i].net > pos[j].net })
	sort.Slice(neg, func(i, j int) bool { return neg[i].net > neg[j].net })
	var tasks []SettlementTask
	i, j := 0, 0
	for i < len(pos) && j < len(neg) {
		c := pos[i]
		d := neg[j]
		amt := math.Min(c.net, d.net)
		ia := int64(math.Round(amt))
		if ia > 0 {
			tasks = append(tasks, SettlementTask{PayerID: d.uid, PayeeID: c.uid, Amount: ia})
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
	sess.Tasks = tasks
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

func (s *Service) CompleteTask(channelID, actorID, otherID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.store[channelID]
	if !ok || !sess.Active {
		return "セッションが開始されていません", nil
	}
	for idx := range sess.Tasks {
		t := &sess.Tasks[idx]
		if t.Completed {
			continue
		}
		if (t.PayerID == actorID && t.PayeeID == otherID) || (t.PayerID == otherID && t.PayeeID == actorID) {
			t.Completed = true
			return fmt.Sprintf("完了しました: <@%s> ↔ <@%s> %d 円", t.PayerID, t.PayeeID, t.Amount), nil
		}
	}
	return "対象のタスクが見つかりません", nil
}
