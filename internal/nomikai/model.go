package nomikai

type Session struct {
    ChannelID    string
    Active       bool
    Participants map[string]*Participant
    Payments     []Payment
    Tasks        []SettlementTask
}

type Participant struct {
    UserID  string
    Weight  float64
    PaidSum int64
}

type Payment struct {
    PayerID string
    Amount  int64
    Memo    string
    Beneficiaries []string // 空なら全参加者対象
}

type SettlementTask struct {
    PayerID   string
    PayeeID   string
    Amount    int64
    Completed bool
}

type SettleResult struct {
    Tasks   []SettlementTask
    Summary string
}
