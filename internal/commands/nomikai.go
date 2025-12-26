package commands

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/nomikai"
)

func HandleNomikai(s *discordgo.Session, i *discordgo.InteractionCreate, svc *nomikai.Service) {
	data := i.ApplicationCommandData()
	if len(data.Options) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "サブコマンドが指定されていません"},
		})
		return
	}

	sub := data.Options[0]
	channelID := i.ChannelID
	userID := i.Member.User.ID

	switch sub.Name {
	case "start":
		// Parse guild ID to int64
		gid, errParse := strconv.ParseInt(i.GuildID, 10, 64)
		if errParse != nil || gid == 0 {
			respondText(s, i, "ギルドIDの取得に失敗しました")
			return
		}
		// Defaults: rounding=1, remainder strategy="organizer"
		err := svc.StartSession(context.Background(), channelID, gid, userID, 1, "organizer")
		respondSimple(s, i, err, "このチャンネルでセッションを開始しました", "既に開始されています")
	case "stop":
		err := svc.StopSession(context.Background(), channelID)
		respondSimple(s, i, err, "セッションを終了しました", "セッションが存在しません")
	case "join":
		err := svc.Join(context.Background(), channelID, userID)
		respondSimple(s, i, err, "参加者として登録しました", "セッションが開始されていません")
	case "member":
		usersOpt := getStringOption(sub.Options, "users")
		if usersOpt == nil {
			respondText(s, i, "users の指定が必要です")
			return
		}
		ids := parseMentionIDs(*usersOpt)
		if len(ids) == 0 {
			respondText(s, i, "ユーザーのメンション/IDを認識できませんでした")
			return
		}
		for _, id := range ids {
			if err := svc.Join(context.Background(), channelID, id); err != nil {
				respondText(s, i, "セッションが開始されていません")
				return
			}
		}
		if len(ids) == 1 {
			respondText(s, i, fmt.Sprintf("<@%s> を参加者に追加しました", ids[0]))
		} else {
			var b strings.Builder
			fmt.Fprintf(&b, "%d 名を参加者に追加しました\n追加: ", len(ids))
			for idx, id := range ids {
				if idx > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "<@%s>", id)
			}
			respondText(s, i, b.String())
		}
	case "weight":
		usersOpt := getStringOption(sub.Options, "users")
		val := getNumberOption(sub.Options, "value")
		if usersOpt == nil || val == nil {
			respondText(s, i, "users と value の指定が必要です")
			return
		}
		ids := parseMentionIDs(*usersOpt)
		if len(ids) == 0 {
			respondText(s, i, "ユーザーのメンション/IDを認識できませんでした")
			return
		}
		var joinedIDs []string
		for _, id := range ids {
			joined, _ := svc.SetWeight(context.Background(), channelID, id, *val)
			if joined {
				joinedIDs = append(joinedIDs, id)
			}
		}
		if len(ids) == 1 {
			msg := fmt.Sprintf("<@%s> の比率を %.2f に設定しました", ids[0], *val)
			if len(joinedIDs) == 1 {
				msg += "\nこのユーザーを参加登録しました"
			}
			respondText(s, i, msg)
		} else {
			msg := fmt.Sprintf("%d 名の比率を %.2f に設定しました", len(ids), *val)
			if len(joinedIDs) > 0 {
				msg += "\n参加登録: "
				for idx, id := range joinedIDs {
					if idx > 0 {
						msg += ", "
					}
					msg += fmt.Sprintf("<@%s>", id)
				}
			}
			respondText(s, i, msg)
		}
	case "pay":
		amtOpt := getIntOption(sub.Options, "amount")
		memoOpt := getStringOption(sub.Options, "memo")
		forOpt := getStringOption(sub.Options, "for")
		if amtOpt == nil {
			respondText(s, i, "金額の指定が必要です")
			return
		}
		memo := ""
		if memoOpt != nil {
			memo = *memoOpt
		}
		payerID := getUserID(data, sub, "payer")
		payer := userID
		if payerID != "" {
			payer = payerID
		}
		var beneficiaries []string
		if forOpt != nil {
			beneficiaries = parseMentionIDs(*forOpt)
		}
		var joined bool
		var benJoined []string
		var err error
		if len(beneficiaries) > 0 {
			joined, benJoined, err = svc.AddPaymentFor(context.Background(), channelID, payer, *amtOpt, memo, beneficiaries)
		} else {
			joined, err = svc.AddPayment(context.Background(), channelID, payer, *amtOpt, memo)
		}
		if err != nil {
			respondText(s, i, err.Error())
			return
		}
		msg := fmt.Sprintf("<@%s> の支払として %d 円を記録しました", payer, *amtOpt)
		if len(beneficiaries) > 0 {
			msg += "\n対象: "
			for idx, id := range beneficiaries {
				if idx > 0 {
					msg += ", "
				}
				msg += fmt.Sprintf("<@%s>", id)
			}
		}
		// Compose join notifications (payer and newly joined beneficiaries)
		var joinIDs []string
		if joined {
			joinIDs = append(joinIDs, payer)
		}
		if len(benJoined) > 0 {
			joinIDs = append(joinIDs, benJoined...)
		}
		if len(joinIDs) > 0 {
			msg += "\n参加登録: "
			for idx, id := range joinIDs {
				if idx > 0 {
					msg += ", "
				}
				msg += fmt.Sprintf("<@%s>", id)
			}
		}
		respondText(s, i, msg)
	case "settle":
		res, err := svc.Settle(context.Background(), channelID)
		if err != nil {
			respondText(s, i, err.Error())
			return
		}
		if len(res.Tasks) == 0 {
			respondText(s, i, "精算は不要です")
			return
		}
		respondText(s, i, res.Summary)
	case "status":
		txt, err := svc.Status(context.Background(), channelID)
		if err != nil {
			respondText(s, i, err.Error())
			return
		}
		respondText(s, i, txt)
	case "memberlist":
		ids, err := svc.Members(context.Background(), channelID)
		if err != nil {
			respondText(s, i, err.Error())
			return
		}
		var b strings.Builder
		fmt.Fprintf(&b, "参加者 (%d名):\n", len(ids))
		for _, id := range ids {
			fmt.Fprintf(&b, "・<@%s>\n", id)
		}
		respondText(s, i, b.String())
	case "remind":
		intervalMinutes := 0
		if opt := getStringOption(sub.Options, "interval"); opt != nil {
			mins, err := parseDHMToMinutes(*opt)
			if err != nil {
				respondText(s, i, err.Error())
				return
			}
			intervalMinutes = mins
		}
		disable := false
		if opt := getStringOption(sub.Options, "state"); opt != nil {
			state := strings.ToLower(strings.TrimSpace(*opt))
			switch state {
			case "on", "enable":
				disable = false
			case "off", "disable":
				disable = true
			case "オン":
				disable = false
			case "オフ":
				disable = true
			default:
				respondText(s, i, "state は on/off で指定してください")
				return
			}
		}
		msg, err := svc.ConfigureReminder(context.Background(), channelID, intervalMinutes, disable, true)
		if err != nil {
			respondText(s, i, err.Error())
			return
		}
		respondText(s, i, msg)
	case "paid":
		amtOpt := getIntOption(sub.Options, "amount")
		if amtOpt == nil || *amtOpt <= 0 {
			respondText(s, i, "amount は正の数で指定してください")
			return
		}
		payee := getUserID(data, sub, "to")
		if payee == "" {
			respondText(s, i, "to の指定が必要です")
			return
		}
		payer := getUserID(data, sub, "payer")
		if payer == "" {
			payer = userID
		}
		memo := ""
		if opt := getStringOption(sub.Options, "memo"); opt != nil {
			memo = *opt
		}
		msg, err := svc.RegisterPayment(context.Background(), channelID, payer, payee, *amtOpt, memo, userID)
		if err != nil {
			respondText(s, i, err.Error())
			return
		}
		respondText(s, i, msg)
	default:
		respondText(s, i, "未知のサブコマンドです")
	}
}

func respondSimple(s *discordgo.Session, i *discordgo.InteractionCreate, err error, ok, ng string) {
	if err != nil {
		respondText(s, i, ng)
		return
	}
	respondText(s, i, ok)
}

func respondText(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
}

func getUserID(data discordgo.ApplicationCommandInteractionData, sub *discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, o := range sub.Options {
		if o.Name != name {
			continue
		}
		// Prefer raw ID from option value
		if id, ok := o.Value.(string); ok && id != "" {
			return id
		}
		// Fallback to resolved user (if available)
		if data.Resolved != nil {
			// When only one user is resolved and this option targets a user, return its ID
			for id := range data.Resolved.Users {
				return id
			}
		}
		// Last resort: try UserValue (may require session; nil is tolerated)
		if u := o.UserValue(nil); u != nil {
			return u.ID
		}
	}
	return ""
}

func getNumberOption(opts []*discordgo.ApplicationCommandInteractionDataOption, name string) *float64 {
	for _, o := range opts {
		if o.Name == name {
			v := o.FloatValue()
			return &v
		}
	}
	return nil
}

func getIntOption(opts []*discordgo.ApplicationCommandInteractionDataOption, name string) *int64 {
	for _, o := range opts {
		if o.Name == name {
			v := o.IntValue()
			return &v
		}
	}
	return nil
}

func getStringOption(opts []*discordgo.ApplicationCommandInteractionDataOption, name string) *string {
	for _, o := range opts {
		if o.Name == name {
			v := o.StringValue()
			return &v
		}
	}
	return nil
}

func getBoolOption(opts []*discordgo.ApplicationCommandInteractionDataOption, name string) *bool {
	for _, o := range opts {
		if o.Name == name {
			v := o.BoolValue()
			return &v
		}
	}
	return nil
}

// no session needed for reading raw ID from options

func parseMentionIDs(text string) []string {
	// Supports <@123>, <@!123>, and raw IDs separated by spaces
	re := regexp.MustCompile(`<@!?([0-9]+)>`)
	var ids []string
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 {
			ids = append(ids, m[1])
		}
	}
	// also allow raw IDs separated by spaces
	for _, tok := range strings.Fields(text) {
		if tok == "" {
			continue
		}
		// if it's pure digits, treat as ID
		if allDigits(tok) {
			ids = append(ids, tok)
		}
	}
	return unique(ids)
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

func unique(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func parseDHMToMinutes(input string) (int, error) {
	s := strings.TrimSpace(strings.ToLower(input))
	if s == "" {
		return 1440, nil
	}
	if allDigits(s) {
		// backward-compatible: treat pure digits as minutes
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("interval の解析に失敗しました: %w", err)
		}
		return int(v), nil
	}

	re := regexp.MustCompile(`(?i)(\d+)([dhm])`)
	matches := re.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("interval は 1d2h3m の形式で指定してください (例: 1d / 2h / 30m / 1d2h3m)")
	}

	var total int64
	pos := 0
	for _, m := range matches {
		if m[0] != pos {
			return 0, fmt.Errorf("interval は 1d2h3m の形式で指定してください (例: 1d2h3m)")
		}
		numStr := s[m[2]:m[3]]
		unit := s[m[4]:m[5]]
		n, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("interval の解析に失敗しました: %w", err)
		}
		switch unit {
		case "d":
			total += n * 24 * 60
		case "h":
			total += n * 60
		case "m":
			total += n
		default:
			return 0, fmt.Errorf("interval は d/h/m のみ対応です")
		}
		pos = m[1]
	}
	if pos != len(s) {
		return 0, fmt.Errorf("interval は 1d2h3m の形式で指定してください (例: 1d2h3m)")
	}
	if total > int64(^uint(0)>>1) {
		return 0, fmt.Errorf("interval が大きすぎます")
	}
	return int(total), nil
}
