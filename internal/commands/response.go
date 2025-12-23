package commands

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/db"
)

func HandleRegisterAsResponse(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	// Get the message from the interaction
	if data.Resolved == nil || len(data.Resolved.Messages) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "メッセージが見つかりませんでした。",
			},
		})
		return
	}

	var message *discordgo.Message
	for _, msg := range data.Resolved.Messages {
		message = msg
		break
	}

	// Show modal to get command name
	customID := fmt.Sprintf("reg_resp:%s", message.ID)
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: customID,
			Title:    "コマンド名を入力",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "command_name",
							Label:       "コマンド名",
							Style:       discordgo.TextInputShort,
							Placeholder: "例: hello",
							Required:    true,
							MaxLength:   50,
						},
					},
				},
			},
		},
	})

	if err != nil {
		log.Printf("Failed to create modal: %v", err)
	}
}

func HandleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, db *db.DB) {
	data := i.ModalSubmitData()
	if !strings.HasPrefix(data.CustomID, "reg_resp:") {
		return
	}

	guildID := ParseGuildID(i.GuildID)
	messageID := strings.TrimPrefix(data.CustomID, "reg_resp:")

	// Get command name from modal
	var commandName string
	for _, component := range data.Components {
		if actionRow, ok := component.(*discordgo.ActionsRow); ok {
			for _, c := range actionRow.Components {
				if input, ok := c.(*discordgo.TextInput); ok && input.CustomID == "command_name" {
					commandName = input.Value
				}
			}
		}
	}

	if commandName == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "コマンド名が入力されていません。",
			},
		})
		return
	}

	// Fetch the original message
	message, err := s.ChannelMessage(i.ChannelID, messageID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "メッセージの取得に失敗しました。",
			},
		})
		return
	}

	// Build response content (message content + attachment URLs)
	responseContent := message.Content
	for _, attachment := range message.Attachments {
		if responseContent != "" {
			responseContent += "\n"
		}
		responseContent += attachment.URL
	}

	// Add command to database
	err = db.AddCommand(context.Background(), guildID, commandName, responseContent)
	var content string
	if err != nil {
		content = "登録に失敗しました。同じ名前のコマンドが既に存在するかもしれません。"
	} else {
		content = fmt.Sprintf("メッセージの内容をコマンド '%s' の返答として登録しました！", commandName)
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}
