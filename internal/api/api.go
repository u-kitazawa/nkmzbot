package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/susu3304/nkmzbot/internal/config"
	"github.com/susu3304/nkmzbot/internal/db"
	"golang.org/x/oauth2"
)

type API struct {
	router       *mux.Router
	db           *db.DB
	config       *config.Config
	oauthConfig  *oauth2.Config
	jwtSecret    []byte
}

type Claims struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	AccessToken string `json:"access_token"`
	jwt.RegisteredClaims
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

// Auth handlers
func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	state := generateRandomString(32)
	url := a.oauthConfig.AuthCodeURL(state)
	
	json.NewEncoder(w).Encode(map[string]string{
		"auth_url": url,
		"state":    state,
	})
}

func (a *API) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := a.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, fmt.Sprintf("token exchange failed: %v", err), http.StatusBadGateway)
		return
	}

	// Get user info
	user, err := a.getDiscordUser(token.AccessToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get user: %v", err), http.StatusBadGateway)
		return
	}

	// Create JWT
	claims := &Claims{
		UserID:      user.ID,
		Username:    user.Username,
		AccessToken: token.AccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := jwtToken.SignedString(a.jwtSecret)
	if err != nil {
		http.Error(w, "failed to create token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":    tokenString,
		"user_id":  user.ID,
		"username": getUsername(user),
	})
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "logged out",
	})
}

// Protected handlers
func (a *API) handleUserGuilds(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*Claims)
	
	guilds, err := a.getDiscordGuilds(claims.AccessToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get guilds: %v", err), http.StatusBadGateway)
		return
	}

	// Get registered guild IDs
	registeredIDs, err := a.db.GetRegisteredGuildIDs(context.Background())
	if err != nil {
		http.Error(w, "failed to get registered guilds", http.StatusInternalServerError)
		return
	}

	// Create a map for quick lookup
	registeredMap := make(map[int64]bool)
	for _, id := range registeredIDs {
		registeredMap[id] = true
	}

	// Filter guilds
	var filtered []DiscordGuild
	for _, guild := range guilds {
		guildID, _ := strconv.ParseInt(guild.ID, 10, 64)
		if registeredMap[guildID] {
			filtered = append(filtered, guild)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

func (a *API) handleListCommands(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*Claims)
	vars := mux.Vars(r)
	guildID, err := strconv.ParseInt(vars["guild_id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid guild_id", http.StatusBadRequest)
		return
	}

	// Verify user has access to guild
	if !a.userHasGuildAccess(claims.AccessToken, guildID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	pattern := r.URL.Query().Get("q")
	commands, err := a.db.ListCommands(context.Background(), guildID, pattern)
	if err != nil {
		http.Error(w, "failed to list commands", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(commands)
}

func (a *API) handleAddCommand(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*Claims)
	vars := mux.Vars(r)
	guildID, err := strconv.ParseInt(vars["guild_id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid guild_id", http.StatusBadRequest)
		return
	}

	// Verify user has access to guild
	if !a.userHasGuildAccess(claims.AccessToken, guildID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Name     string `json:"name"`
		Response string `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := a.db.AddCommand(context.Background(), guildID, req.Name, req.Response); err != nil {
		http.Error(w, "failed to add command", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "command added",
	})
}

func (a *API) handleUpdateCommand(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*Claims)
	vars := mux.Vars(r)
	guildID, err := strconv.ParseInt(vars["guild_id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid guild_id", http.StatusBadRequest)
		return
	}
	name := vars["name"]

	// Verify user has access to guild
	if !a.userHasGuildAccess(claims.AccessToken, guildID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := a.db.UpdateCommand(context.Background(), guildID, name, req.Response); err != nil {
		http.Error(w, "failed to update command", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "command updated",
	})
}

func (a *API) handleDeleteCommand(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*Claims)
	vars := mux.Vars(r)
	guildID, err := strconv.ParseInt(vars["guild_id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid guild_id", http.StatusBadRequest)
		return
	}
	name := vars["name"]

	// Verify user has access to guild
	if !a.userHasGuildAccess(claims.AccessToken, guildID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := a.db.RemoveCommand(context.Background(), guildID, name); err != nil {
		http.Error(w, "failed to delete command", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "command deleted",
	})
}

func (a *API) handleBulkDeleteCommands(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value("claims").(*Claims)
	vars := mux.Vars(r)
	guildID, err := strconv.ParseInt(vars["guild_id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid guild_id", http.StatusBadRequest)
		return
	}

	// Verify user has access to guild
	if !a.userHasGuildAccess(claims.AccessToken, guildID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Names []string `json:"names"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var errors []string
	successCount := 0
	for _, name := range req.Names {
		if err := a.db.RemoveCommand(context.Background(), guildID, name); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to delete '%s': %v", name, err))
		} else {
			successCount++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"deleted": successCount,
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}
	json.NewEncoder(w).Encode(response)
}

// Middleware
func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return a.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "claims", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Helper functions
func (a *API) userHasGuildAccess(accessToken string, guildID int64) bool {
	guilds, err := a.getDiscordGuilds(accessToken)
	if err != nil {
		return false
	}

	for _, guild := range guilds {
		id, _ := strconv.ParseInt(guild.ID, 10, 64)
		if id == guildID {
			return true
		}
	}
	return false
}
