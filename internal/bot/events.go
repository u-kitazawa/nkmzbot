package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
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
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.handleApplicationCommandAutocomplete(s, i)
	case discordgo.InteractionModalSubmit:
		b.handleModalSubmit(s, i)
	}
}

func (b *Bot) handleApplicationCommandAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if data.Name != "nomikai" {
		return
	}
	if len(data.Options) == 0 {
		return
	}
	sub := data.Options[0]
	if sub.Name != "seisan" {
		return
	}

	// Find focused option in the subcommand.
	focusedName := ""
	userInput := ""
	for _, opt := range sub.Options {
		if opt.Focused {
			focusedName = opt.Name
			userInput = opt.StringValue()
			break
		}
	}
	if focusedName != "amount" {
		return
	}

	payeeID := ""
	payerID := ""
	for _, opt := range sub.Options {
		switch opt.Name {
		case "to":
			if id, ok := opt.Value.(string); ok {
				payeeID = id
			}
		case "payer":
			if id, ok := opt.Value.(string); ok {
				payerID = id
			}
		}
	}
	if payerID == "" && i.Member != nil && i.Member.User != nil {
		payerID = i.Member.User.ID
	}

	choices := []*discordgo.ApplicationCommandOptionChoice{
		{Name: "all（未払い全額）", Value: "all"},
	}

	// If we can compute outstanding amount for the pair, also offer it as a one-click numeric choice.
	if payerID != "" && payeeID != "" {
		ev, err := b.db.ActiveEventByChannel(context.Background(), i.ChannelID)
		if err == nil && ev != nil {
			out, err := b.db.OutstandingSettlementAmount(context.Background(), ev.ID, payerID, payeeID)
			if err == nil && out > 0 {
				choices = append([]*discordgo.ApplicationCommandOptionChoice{
					{Name: fmt.Sprintf("%d（未払い全額）", out), Value: strconv.FormatInt(out, 10)},
				}, choices...)
			}
		}
	}

	// If user typed something, echo it as a choice so they can commit it quickly.
	if strings.TrimSpace(userInput) != "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: userInput, Value: userInput})
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
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
		commands.HandleList(s, i, b.db, b.config)
	case "nomikai":
		commands.HandleNomikai(s, i, b.nomikai)
	case "guess":
		commands.HandleGuess(s, i, b.guess)
	case "jikan":
		commands.HandleJikan(s, i, b.nomikai, b.db)
	case "Register as Response":
		commands.HandleRegisterAsResponse(s, i)
	}
}

func (b *Bot) handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	commands.HandleModalSubmit(s, i, b.db)
}
