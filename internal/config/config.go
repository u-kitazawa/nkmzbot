package config

import (
	"fmt"
	"net/url"
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
	WebBind       string
	WebUIBaseURL  string

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
		JWTSecret:           getEnvDefault("JWT_SECRET", "dev-only-change-me"),
	}

	// Extract base URL from redirect URI
	cfg.WebUIBaseURL = extractBaseURL(cfg.DiscordRedirectURI)

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

func extractBaseURL(redirectURI string) string {
	// Extract base URL from redirect URI using url.Parse
	// e.g., "http://localhost:3000/api/auth/callback" -> "http://localhost:3000"
	parsed, err := url.Parse(redirectURI)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "http://localhost:3000"
	}
	
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}
