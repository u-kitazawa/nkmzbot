package bot

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/db"
	"github.com/susu3304/nkmzbot/internal/nomikai"
)

type Bot struct {
	session *discordgo.Session
	db      *db.DB
	nomikai *nomikai.Service
}

func New(token string, database *db.DB) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	// Enable state tracking to maintain session across reconnects
	session.StateEnabled = true
	// Enable automatic reconnection on errors
	session.ShouldReconnectOnError = true

	bot := &Bot{
		session: session,
		db:      database,
		nomikai: nomikai.NewService(),
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
