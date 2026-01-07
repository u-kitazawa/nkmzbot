package bot

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/commands"
	"github.com/susu3304/nkmzbot/internal/config"
	"github.com/susu3304/nkmzbot/internal/db"
	"github.com/susu3304/nkmzbot/internal/guess"
	"github.com/susu3304/nkmzbot/internal/nomikai"
)

type Bot struct {
	session  *discordgo.Session
	db       *db.DB
	nomikai  *nomikai.Service
	guess    *guess.Service
	reminder *reminderWorker
	config   *config.Config
}

func New(token string, database *db.DB, cfg *config.Config) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}

	// Keep the overall request timeout modest, but avoid long hangs by tuning transport-level timeouts.
	// (We prefer fixing stalls over simply increasing the global timeout.)
	if session.Client == nil {
		session.Client = &http.Client{}
	}
	session.Client.Timeout = 20 * time.Second
	session.Client.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	bot := &Bot{
		session: session,
		db:      database,
		nomikai: nomikai.NewService(database),
		guess:   guess.NewService(database),
		config:  cfg,
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
	
	// Restore scheduled tasks from database
	ctx := context.Background()
	if err := commands.RestoreScheduledTasks(ctx, b.session, b.nomikai, b.db); err != nil {
		log.Printf("Warning: failed to restore scheduled tasks: %v", err)
	}
	
	b.reminder.start()
	return nil
}

func (b *Bot) Stop() error {
	if b.reminder != nil {
		b.reminder.stop()
	}
	return b.session.Close()
}
