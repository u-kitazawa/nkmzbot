package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/db"
)

type Bot struct {
	session *discordgo.Session
	db      *db.DB
}

func New(token string, database *db.DB) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	bot := &Bot{
		session: session,
		db:      database,
	}

	// Register event handlers
	session.AddHandler(bot.onReady)
	session.AddHandler(bot.onGuildCreate)
	session.AddHandler(bot.onMessageCreate)
	session.AddHandler(bot.onInteractionCreate)

	session.Identify.Intents = discordgo.IntentsAll

	return bot, nil
}

func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open discord session: %w", err)
	}
	log.Println("Discord bot is running")
	return nil
}

func (b *Bot) Stop() error {
	return b.session.Close()
}

func (b *Bot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("%s is connected!", event.User.Username)

	// Register commands for all guilds
	for _, guild := range event.Guilds {
		if err := b.registerGuildCommands(guild.ID); err != nil {
			log.Printf("Failed to register commands for guild %s: %v", guild.ID, err)
		}
	}
}

func (b *Bot) onGuildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	log.Printf("Guild available/joined: %s (id=%s) — ensuring commands", event.Name, event.ID)
	if err := b.registerGuildCommands(event.ID); err != nil {
		log.Printf("Failed to register commands for guild %s: %v", event.ID, err)
	}
}

func (b *Bot) registerGuildCommands(guildID string) error {
	commands := []*discordgo.ApplicationCommand{
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

	// Delete existing commands and register new ones
	_, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, guildID, commands)
	if err != nil {
		return err
	}

	log.Printf("Registered application commands for guild %s", guildID)
	return nil
}

func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot messages
	if m.Author.Bot {
		return
	}

	content := strings.TrimSpace(m.Content)
	if strings.HasPrefix(content, "!") && len(content) > 1 {
		cmdName := content[1:]
		if m.GuildID != "" {
			guildID := parseGuildID(m.GuildID)
			cmd, err := b.db.GetCommand(context.Background(), guildID, cmdName)
			if err == nil && cmd != nil {
				s.ChannelMessageSend(m.ChannelID, cmd.Response)
			}
		}
	}
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleApplicationCommand(s, i)
	case discordgo.InteractionModalSubmit:
		b.handleModalSubmit(s, i)
	}
}

func (b *Bot) handleApplicationCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	guildID := parseGuildID(i.GuildID)

	switch data.Name {
	case "add":
		b.handleAddCommand(s, i, guildID, data)
	case "remove":
		b.handleRemoveCommand(s, i, guildID, data)
	case "update":
		b.handleUpdateCommand(s, i, guildID, data)
	case "list":
		b.handleListCommand(s, i, guildID)
	case "Register as Response":
		b.handleRegisterAsResponse(s, i, guildID, data)
	}
}

func (b *Bot) handleAddCommand(s *discordgo.Session, i *discordgo.InteractionCreate, guildID int64, data discordgo.ApplicationCommandInteractionData) {
	options := data.Options
	var name, response string
	for _, opt := range options {
		if opt.Name == "name" {
			name = opt.StringValue()
		} else if opt.Name == "response" {
			response = opt.StringValue()
		}
	}

	err := b.db.AddCommand(context.Background(), guildID, name, response)
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

func (b *Bot) handleRemoveCommand(s *discordgo.Session, i *discordgo.InteractionCreate, guildID int64, data discordgo.ApplicationCommandInteractionData) {
	options := data.Options
	var name string
	for _, opt := range options {
		if opt.Name == "name" {
			name = opt.StringValue()
		}
	}

	err := b.db.RemoveCommand(context.Background(), guildID, name)
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

func (b *Bot) handleUpdateCommand(s *discordgo.Session, i *discordgo.InteractionCreate, guildID int64, data discordgo.ApplicationCommandInteractionData) {
	options := data.Options
	var name, response string
	for _, opt := range options {
		if opt.Name == "name" {
			name = opt.StringValue()
		} else if opt.Name == "response" {
			response = opt.StringValue()
		}
	}

	err := b.db.UpdateCommand(context.Background(), guildID, name, response)
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

func (b *Bot) handleListCommand(s *discordgo.Session, i *discordgo.InteractionCreate, guildID int64) {
	commands, err := b.db.ListCommands(context.Background(), guildID, "")
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

func (b *Bot) handleRegisterAsResponse(s *discordgo.Session, i *discordgo.InteractionCreate, guildID int64, data discordgo.ApplicationCommandInteractionData) {
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

func (b *Bot) handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	if !strings.HasPrefix(data.CustomID, "reg_resp:") {
		return
	}

	guildID := parseGuildID(i.GuildID)
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
	err = b.db.AddCommand(context.Background(), guildID, commandName, responseContent)
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

func parseGuildID(guildID string) int64 {
	var id int64
	fmt.Sscanf(guildID, "%d", &id)
	return id
}

func boolPtr(b bool) *bool {
	return &b
}
