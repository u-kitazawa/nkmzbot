package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/config"
	"github.com/susu3304/nkmzbot/internal/db"
)

func HandleList(s *discordgo.Session, i *discordgo.InteractionCreate, db *db.DB, cfg *config.Config) {
	data := i.ApplicationCommandData()
	
	// Check if there's a subcommand
	if len(data.Options) > 0 && data.Options[0].Name == "web" {
		// Handle /list web subcommand
		guildID := i.GuildID
		if guildID == "" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "このコマンドはギルド内でのみ使用できます。",
				},
			})
			return
		}
		
		webuiURL := fmt.Sprintf("%s/guilds/%s", cfg.WebUIBaseURL, guildID)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("WebUI URL: %s", webuiURL),
			},
		})
		return
	}
	
	// Original /list command (no subcommand)
	guildID := ParseGuildID(i.GuildID)
	commands, err := db.ListCommands(context.Background(), guildID, "")
	if err != nil || len(commands) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "コマンドは登録されていません。",
			},
		})
		return
	}

	// Build command list
	var entries []string
	for _, cmd := range commands {
		entries = append(entries, fmt.Sprintf("!%s: %s", cmd.Name, cmd.Response))
	}

	// Split into 2000 character chunks
	var buffer strings.Builder
	for _, entry := range entries {
		if buffer.Len()+len(entry)+1 > 2000 {
			s.ChannelMessageSend(i.ChannelID, buffer.String())
			buffer.Reset()
		}
		if buffer.Len() > 0 {
			buffer.WriteString("\n")
		}
		buffer.WriteString(entry)
	}

	if buffer.Len() > 0 {
		s.ChannelMessageSend(i.ChannelID, buffer.String())
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "コマンド一覧を送信しました。",
		},
	})
}
