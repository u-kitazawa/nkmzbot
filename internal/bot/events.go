package bot

import (
	"context"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/commands"
)

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
	cmds := commands.GetCommands()
	// Delete existing commands and register new ones
	_, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, guildID, cmds)
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
			guildID := commands.ParseGuildID(m.GuildID)
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

	switch data.Name {
	case "add":
		commands.HandleAdd(s, i, b.db)
	case "remove":
		commands.HandleRemove(s, i, b.db)
	case "update":
		commands.HandleUpdate(s, i, b.db)
	case "list":
		commands.HandleList(s, i, b.db)
	case "nomikai":
		commands.HandleNomikai(s, i, b.nomikai)
	case "Register as Response":
		commands.HandleRegisterAsResponse(s, i)
	case "join":
		commands.HandleJoin(s, i, b.voiceManager)
	case "leave":
		commands.HandleLeave(s, i, b.voiceManager, b.transcriber)
	}
}

func (b *Bot) handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	commands.HandleModalSubmit(s, i, b.db)
}
