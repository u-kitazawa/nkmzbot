package commands

import (
	"fmt"
	"regexp"
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
        err := svc.StartSession(channelID)
        respondSimple(s, i, err, "このチャンネルでセッションを開始しました", "既に開始されています")
    case "stop":
        err := svc.StopSession(channelID)
        respondSimple(s, i, err, "セッションを終了しました", "セッションが存在しません")
    case "join":
        err := svc.Join(channelID, userID)
        respondSimple(s, i, err, "参加者として登録しました", "セッションが開始されていません")
    case "member":
        uid := getUserID(data, sub, "user")
        if uid == "" {
            respondText(s, i, "ユーザーが指定されていません")
            return
        }
        err := svc.Join(channelID, uid)
        if err != nil {
            respondText(s, i, "セッションが開始されていません")
            return
        }
        respondText(s, i, fmt.Sprintf("<@%s> を参加者に追加しました", uid))
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
            joined, _ := svc.SetWeight(channelID, id, *val)
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
                    if idx > 0 { msg += ", " }
                    msg += fmt.Sprintf("<@%s>", id)
                }
            }
            respondText(s, i, msg)
        }
    case "pay":
        amtOpt := getIntOption(sub.Options, "amount")
        memoOpt := getStringOption(sub.Options, "memo")
        if amtOpt == nil {
            respondText(s, i, "金額の指定が必要です")
            return
        }
        memo := ""
        if memoOpt != nil {
            memo = *memoOpt
        }
        joined, err := svc.AddPayment(channelID, userID, *amtOpt, memo)
        if err != nil {
            respondText(s, i, err.Error())
            return
        }
        msg := fmt.Sprintf("%d 円を記録しました", *amtOpt)
        if joined {
            msg += "\nこのユーザーを参加登録しました"
        }
        respondText(s, i, msg)
    case "settle":
        res, err := svc.Settle(channelID)
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
        txt, err := svc.Status(channelID)
        if err != nil {
            respondText(s, i, err.Error())
            return
        }
        respondText(s, i, txt)
    case "memberlist":
        ids, err := svc.Members(channelID)
        if err != nil {
            respondText(s, i, err.Error())
            return
        }
        if len(ids) == 0 {
            respondText(s, i, "参加者がいません")
            return
        }
        var b strings.Builder
        fmt.Fprintf(&b, "参加者 (%d名):\n", len(ids))
        for _, id := range ids {
            fmt.Fprintf(&b, "・<@%s>\n", id)
        }
        respondText(s, i, b.String())
    case "done":
        uid := getUserID(data, sub, "user")
        if uid == "" {
            respondText(s, i, "相手の指定が必要です")
            return
        }
        msg, err := svc.CompleteTask(channelID, userID, uid)
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
