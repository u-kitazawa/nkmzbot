package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

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
