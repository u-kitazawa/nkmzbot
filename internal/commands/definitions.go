package commands

import "github.com/bwmarrin/discordgo"

func GetCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:           "add",
			Description:    "新しいコマンドを追加します",
			DMPermission:   boolPtr(false),
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
			Name:           "remove",
			Description:    "コマンドを削除します",
			DMPermission:   boolPtr(false),
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
			Name:           "update",
			Description:    "コマンドを更新します",
			DMPermission:   boolPtr(false),
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
			Name:           "list",
			Description:    "登録されているコマンド一覧を表示します",
			DMPermission:   boolPtr(false),
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
