package commands

import "github.com/bwmarrin/discordgo"

func GetCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:         "add",
			Description:  "新しいコマンドを追加します",
			DMPermission: boolPtr(false),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "コマンド名",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "response",
					Description: "返答内容",
					Required:    true,
				},
			},
		},
		{
			Name:         "remove",
			Description:  "コマンドを削除します",
			DMPermission: boolPtr(false),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "削除するコマンド名",
					Required:    true,
				},
			},
		},
		{
			Name:         "update",
			Description:  "コマンドを更新します",
			DMPermission: boolPtr(false),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "更新するコマンド名",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "response",
					Description: "新しい返答内容",
					Required:    true,
				},
			},
		},
		{
			Name:         "list",
			Description:  "登録されているコマンド一覧を表示します",
			DMPermission: boolPtr(false),
		},
		{
			Name:         "nomikai",
			Description:  "飲み会割り勘セッションを操作します",
			DMPermission: boolPtr(false),
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "start",
					Description: "このチャンネルでセッションを開始",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "stop",
					Description: "このチャンネルのセッションを終了",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "join",
					Description: "自分を参加者に追加",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "member",
					Description: "指定ユーザーを参加者に追加",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "users",
							Description: "追加するユーザー（メンション/IDをスペース区切り。単一も可）",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "weight",
					Description: "参加者の比率を設定",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "users",
							Description: "対象ユーザー（メンション/IDをスペース区切り。単一も可）",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionNumber,
							Name:        "value",
							Description: "比率 (例: 1.5)",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "tatekae",
					Description: "立替（支出）を記録（負額も可）",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "amount",
							Description: "金額（円）",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "payer",
							Description: "支払者（未指定なら自分）",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "for",
							Description: "対象ユーザー（メンション/ID。スペース区切りで複数可）",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "memo",
							Description: "メモ",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "settle",
					Description: "ネット精算を計算",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "status",
					Description: "現在の状況を表示",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "memberlist",
					Description: "参加中のメンバーを表示",
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remind",
					Description: "未払いタスクの定期リマインドを設定し即時送信",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "interval",
							Description: "リマインド間隔 (例: 1d2h3m / デフォルト1d / 最小1m)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "state",
							Description: "on/off (on=有効, off=停止)",
							Required:    false,
							Choices: []*discordgo.ApplicationCommandOptionChoice{
								{Name: "on", Value: "on"},
								{Name: "off", Value: "off"},
							},
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "seisan",
					Description: "精算の支払いを登録して未払いを減らす",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "to",
							Description: "受け取り側",
							Required:    true,
						},
						{
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "amount",
							Description:  "支払った金額 (円) / all=未払い全額",
							Required:     true,
							Autocomplete: true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "payer",
							Description: "支払者 (未指定なら自分)",
							Required:    false,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "memo",
							Description: "メモ",
							Required:    false,
						},
					},
				},
			},
		},
		{
			Name: "Register as Response",
			Type: discordgo.MessageApplicationCommand,
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}
