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
        ChannelID:   channelID,
        Active:      true,
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

func (s *Service) SetWeight(channelID, userID string, w float64) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    sess, ok := s.store[channelID]
    if !ok || !sess.Active {
        return errors.New("セッションが開始されていません")
    }
    p, exists := sess.Participants[userID]
    if !exists {
        p = &Participant{UserID: userID, Weight: 1.0}
        sess.Participants[userID] = p
    }
    if w <= 0 {
        w = 0
    }
    p.Weight = w
    return nil
}

func (s *Service) AddPayment(channelID, userID string, amount int64, memo string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    sess, ok := s.store[channelID]
    if !ok || !sess.Active {
        return errors.New("セッションが開始されていません")
    }
    p, exists := sess.Participants[userID]
    if !exists {
        p = &Participant{UserID: userID, Weight: 1.0}
        sess.Participants[userID] = p
    }
    if p.PaidSum+amount < 0 {
        return errors.New("訂正額により合計が負になります")
    }
    p.PaidSum += amount
    sess.Payments = append(sess.Payments, Payment{PayerID: userID, Amount: amount, Memo: memo})
    return nil
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
    type row struct{ uid string; w float64; paid int64 }
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
    var total int64
    var wsum float64
    for _, p := range sess.Participants {
        total += p.PaidSum
        wsum += p.Weight
    }
    if wsum == 0 {
        return &SettleResult{Tasks: nil, Summary: "全員の比率が0です"}, nil
    }
    type bal struct{ uid string; net float64 }
    var pos, neg []bal
    for uid, p := range sess.Participants {
        ci := float64(total) * (p.Weight / wsum)
        net := float64(p.PaidSum) - ci
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
