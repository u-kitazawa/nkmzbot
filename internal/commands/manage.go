package commands

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/db"
)

func HandleAdd(s *discordgo.Session, i *discordgo.InteractionCreate, db *db.DB) {
	data := i.ApplicationCommandData()
	guildID := ParseGuildID(i.GuildID)

	options := data.Options
	var name, response string
	for _, opt := range options {
		if opt.Name == "name" {
			name = opt.StringValue()
		} else if opt.Name == "response" {
			response = opt.StringValue()
		}
	}

	err := db.AddCommand(context.Background(), guildID, name, response)
	var content string
	if err != nil {
		content = "追加に失敗しました。"
	} else {
		content = fmt.Sprintf("コマンド '%s' を追加しました。", name)
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}

func HandleRemove(s *discordgo.Session, i *discordgo.InteractionCreate, db *db.DB) {
	data := i.ApplicationCommandData()
	guildID := ParseGuildID(i.GuildID)

	options := data.Options
	var name string
	for _, opt := range options {
		if opt.Name == "name" {
			name = opt.StringValue()
		}
	}

	err := db.RemoveCommand(context.Background(), guildID, name)
	var content string
	if err != nil {
		content = "そのコマンドは存在しません。"
	} else {
		content = fmt.Sprintf("コマンド '%s' を削除しました。", name)
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}

func HandleUpdate(s *discordgo.Session, i *discordgo.InteractionCreate, db *db.DB) {
	data := i.ApplicationCommandData()
	guildID := ParseGuildID(i.GuildID)

	options := data.Options
	var name, response string
	for _, opt := range options {
		if opt.Name == "name" {
			name = opt.StringValue()
		} else if opt.Name == "response" {
			response = opt.StringValue()
		}
	}

	err := db.UpdateCommand(context.Background(), guildID, name, response)
	var content string
	if err != nil {
		content = "そのコマンドは存在しません。"
	} else {
		content = fmt.Sprintf("コマンド '%s' を更新しました。", name)
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}
