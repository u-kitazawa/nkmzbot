package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/susu3304/nkmzbot/internal/api"
	"github.com/susu3304/nkmzbot/internal/bot"
	"github.com/susu3304/nkmzbot/internal/config"
	"github.com/susu3304/nkmzbot/internal/db"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	database, err := db.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run migrations
	if err := database.RunMigrations(context.Background()); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize Discord bot
	discordBot, err := bot.New(cfg.DiscordToken, database)
	if err != nil {
		log.Fatalf("Failed to create discord bot: %v", err)
	}

	// Initialize API server
	apiServer := api.New(cfg, database)

	// Start Discord bot
	if err := discordBot.Start(); err != nil {
		log.Fatalf("Failed to start discord bot: %v", err)
	}
	defer discordBot.Stop()

	// Start API server
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	// Wait for signal to stop
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
}
