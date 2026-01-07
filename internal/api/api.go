package api

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/susu3304/nkmzbot/internal/config"
	"github.com/susu3304/nkmzbot/internal/db"
	"golang.org/x/oauth2"
)

type API struct {
	router      *mux.Router
	db          *db.DB
	config      *config.Config
	oauthConfig *oauth2.Config
	jwtSecret   []byte
}

func New(cfg *config.Config, database *db.DB) *API {
	api := &API{
		router:    mux.NewRouter(),
		db:        database,
		config:    cfg,
		jwtSecret: []byte(cfg.JWTSecret),
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.DiscordClientID,
			ClientSecret: cfg.DiscordClientSecret,
			RedirectURL:  cfg.DiscordRedirectURI,
			Scopes:       []string{"identify", "guilds"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://discord.com/api/oauth2/authorize",
				TokenURL: "https://discord.com/api/oauth2/token",
			},
		},
	}

	api.setupRoutes()
	return api
}

func (a *API) setupRoutes() {
	// Auth endpoints
	a.router.HandleFunc("/api/auth/login", a.handleLogin).Methods("GET")
	a.router.HandleFunc("/api/auth/callback", a.handleCallback).Methods("GET")
	a.router.HandleFunc("/api/auth/logout", a.handleLogout).Methods("POST")

	// Public endpoints
	a.router.HandleFunc("/api/public/guilds/{guild_id}/commands", a.handlePublicListCommands).Methods("GET")
	
	// Web interface
	a.router.HandleFunc("/", a.handleWebInterface).Methods("GET")
	a.router.HandleFunc("/guilds/{guild_id}", a.handleWebInterface).Methods("GET")

	// Protected endpoints
	protected := a.router.PathPrefix("/api").Subrouter()
	protected.Use(a.authMiddleware)

	protected.HandleFunc("/user/guilds", a.handleUserGuilds).Methods("GET")
	protected.HandleFunc("/guilds/{guild_id}/commands", a.handleListCommands).Methods("GET")
	protected.HandleFunc("/guilds/{guild_id}/commands", a.handleAddCommand).Methods("POST")
	protected.HandleFunc("/guilds/{guild_id}/commands/{name}", a.handleUpdateCommand).Methods("PUT")
	protected.HandleFunc("/guilds/{guild_id}/commands/{name}", a.handleDeleteCommand).Methods("DELETE")
	protected.HandleFunc("/guilds/{guild_id}/commands/bulk-delete", a.handleBulkDeleteCommands).Methods("POST")
}

func (a *API) Start() error {
	// Setup CORS - allow all origins for development, restrict in production
	// Note: When AllowedOrigins is "*", AllowCredentials must be false for security
	corsOptions := cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false, // Set to false for security when using wildcard origin
	}

	// For production, set specific origins and enable credentials:
	// Example: AllowedOrigins: []string{"https://yourdomain.com"}, AllowCredentials: true

	handler := cors.New(corsOptions).Handler(a.router)

	log.Printf("API server listening on http://%s", a.config.WebBind)
	return http.ListenAndServe(a.config.WebBind, handler)
}
