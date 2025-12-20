package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
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

	// OpenAI
	OpenAIKey string

	// Session
	JWTSecret string
}

func Load() (*Config, error) {
	// Load environment variables from .env if present (non-fatal if missing)
	_ = godotenv.Load()

	cfg := &Config{
		DiscordToken:        os.Getenv("DISCORD_TOKEN"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		WebBind:             getEnvDefault("WEB_BIND", "0.0.0.0:3000"),
		DiscordClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		DiscordClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURI:  getEnvDefault("DISCORD_REDIRECT_URI", "http://localhost:3000/api/auth/callback"),
		OpenAIKey:           os.Getenv("OPENAI_API_KEY"),
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
	// OpenAI key is optional for now, or check if required?
	// User said it requires OPENAI_API_KEY. I won't error if missing to allow bot to start without it if voice is unused, 
	// but strictly speaking, features won't work. I'll stick to non-fatal or maybe warn.
	// For now, I won't adding validation to keep it simple.

	return cfg, nil
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
