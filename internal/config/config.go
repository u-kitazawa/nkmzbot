package config

import (
	"fmt"
	"os"
)

type Config struct {
	// Discord Bot
	DiscordToken string

	// Discord OAuth2
	DiscordClientID     string
	DiscordClientSecret string
	DiscordRedirectURI  string

	// Database
	DatabaseURL string

	// Web Server
	WebBind string

	// Session
	JWTSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		DiscordToken:        os.Getenv("DISCORD_TOKEN"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		WebBind:             getEnvDefault("WEB_BIND", "0.0.0.0:3000"),
		DiscordClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		DiscordClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURI:  getEnvDefault("DISCORD_REDIRECT_URI", "http://localhost:3000/api/auth/callback"),
		JWTSecret:           getEnvDefault("JWT_SECRET", "dev-only-change-me"),
	}

	if cfg.DiscordToken == "" {
		return nil, fmt.Errorf("DISCORD_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.DiscordClientID == "" {
		return nil, fmt.Errorf("DISCORD_CLIENT_ID is required")
	}
	if cfg.DiscordClientSecret == "" {
		return nil, fmt.Errorf("DISCORD_CLIENT_SECRET is required")
	}

	return cfg, nil
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
