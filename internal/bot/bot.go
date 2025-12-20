package bot

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/config"
	"github.com/susu3304/nkmzbot/internal/db"
	"github.com/susu3304/nkmzbot/internal/nomikai"
	"github.com/susu3304/nkmzbot/internal/transcribe"
	"github.com/susu3304/nkmzbot/internal/voice"
)

type Bot struct {
	session      *discordgo.Session
	db           *db.DB
	nomikai      *nomikai.Service
	voiceManager *voice.Manager
	transcriber  *transcribe.Client
}

func New(cfg *config.Config, database *db.DB) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	bot := &Bot{
		session:      session,
		db:           database,
		nomikai:      nomikai.NewService(),
		voiceManager: voice.NewManager(),
		transcriber:  transcribe.NewClient(cfg.OpenAIKey),
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
