package bot

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/db"
	"github.com/susu3304/nkmzbot/internal/nomikai"
)

type Bot struct {
	session  *discordgo.Session
	db       *db.DB
	nomikai  *nomikai.Service
	reminder *reminderWorker
}

func New(token string, database *db.DB) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	bot := &Bot{
		session: session,
		db:      database,
		nomikai: nomikai.NewService(database),
	}
	bot.reminder = newReminderWorker(session, database, bot.nomikai)

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
	b.reminder.start()
	return nil
}

func (b *Bot) Stop() error {
	if b.reminder != nil {
		b.reminder.stop()
	}
	return b.session.Close()
}
