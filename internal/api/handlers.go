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

// Public handlers
func (a *API) handlePublicListCommands(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	guildID, err := strconv.ParseInt(vars["guild_id"], 10, 64)
	if err != nil {
		http.Error(w, "invalid guild_id", http.StatusBadRequest)
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

func (a *API) handleWebInterface(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>nkmzbot - コマンド一覧</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        header {
            text-align: center;
            color: white;
            margin-bottom: 40px;
        }
        h1 {
            font-size: 2.5rem;
            margin-bottom: 10px;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.3);
        }
        .subtitle {
            font-size: 1.1rem;
            opacity: 0.9;
        }
        .search-box {
            background: white;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.2);
            margin-bottom: 30px;
        }
        .input-group {
            display: flex;
            gap: 10px;
            margin-bottom: 15px;
        }
        input[type="text"] {
            flex: 1;
            padding: 12px 20px;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            font-size: 1rem;
            transition: border-color 0.3s;
        }
        input[type="text"]:focus {
            outline: none;
            border-color: #667eea;
        }
        button {
            padding: 12px 30px;
            background: #667eea;
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 1rem;
            cursor: pointer;
            transition: background 0.3s;
        }
        button:hover {
            background: #5568d3;
        }
        .stats {
            display: flex;
            gap: 20px;
            justify-content: center;
            color: #666;
            font-size: 0.9rem;
        }
        .commands-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
            gap: 20px;
        }
        .command-card {
            background: white;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 5px 15px rgba(0,0,0,0.1);
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .command-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 10px 25px rgba(0,0,0,0.15);
        }
        .command-name {
            font-size: 1.2rem;
            font-weight: bold;
            color: #667eea;
            margin-bottom: 10px;
            word-break: break-word;
        }
        .command-response {
            color: #555;
            line-height: 1.6;
            white-space: pre-wrap;
            word-break: break-word;
        }
        .loading {
            text-align: center;
            color: white;
            font-size: 1.2rem;
            padding: 40px;
        }
        .error {
            background: #ff5252;
            color: white;
            padding: 20px;
            border-radius: 10px;
            text-align: center;
            margin-bottom: 20px;
        }
        .no-commands {
            text-align: center;
            color: white;
            font-size: 1.2rem;
            padding: 40px;
            background: rgba(255,255,255,0.1);
            border-radius: 10px;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>🤖 nkmzbot</h1>
            <p class="subtitle">コマンド一覧</p>
        </header>

        <div class="search-box">
            <div class="input-group">
                <input type="text" id="guildId" placeholder="Guild ID を入力してください" value="">
                <input type="text" id="searchQuery" placeholder="コマンドを検索...">
                <button onclick="loadCommands()">検索</button>
            </div>
            <div class="stats">
                <span id="commandCount">コマンド数: -</span>
                <span id="guildInfo"></span>
            </div>
        </div>

        <div id="error"></div>
        <div id="loading" class="loading" style="display: none;">読み込み中...</div>
        <div id="commands" class="commands-grid"></div>
    </div>

    <script>
        // Get guild ID from URL path
        const pathMatch = window.location.pathname.match(/\/guilds\/(\d+)/);
        if (pathMatch) {
            document.getElementById('guildId').value = pathMatch[1];
            loadCommands();
        }

        async function loadCommands() {
            const guildId = document.getElementById('guildId').value.trim();
            const searchQuery = document.getElementById('searchQuery').value.trim();
            
            if (!guildId) {
                showError('Guild ID を入力してください');
                return;
            }

            // Update URL without reload
            if (window.location.pathname !== '/guilds/' + guildId) {
                window.history.pushState({}, '', '/guilds/' + guildId);
            }

            const loading = document.getElementById('loading');
            const commandsDiv = document.getElementById('commands');
            const errorDiv = document.getElementById('error');
            
            errorDiv.innerHTML = '';
            loading.style.display = 'block';
            commandsDiv.innerHTML = '';

            try {
                let url = '/api/public/guilds/' + guildId + '/commands';
                if (searchQuery) {
                    url += '?q=' + encodeURIComponent(searchQuery);
                }

                const response = await fetch(url);
                if (!response.ok) {
                    throw new Error('コマンドの取得に失敗しました');
                }

                const commands = await response.json();
                loading.style.display = 'none';

                if (commands.length === 0) {
                    commandsDiv.innerHTML = '<div class="no-commands">コマンドが見つかりませんでした</div>';
                    document.getElementById('commandCount').textContent = 'コマンド数: 0';
                    return;
                }

                document.getElementById('commandCount').textContent = 'コマンド数: ' + commands.length;
                document.getElementById('guildInfo').textContent = 'Guild ID: ' + guildId;

                commands.forEach(cmd => {
                    const card = document.createElement('div');
                    card.className = 'command-card';
                    card.innerHTML = 
                        '<div class="command-name">!' + escapeHtml(cmd.name) + '</div>' +
                        '<div class="command-response">' + escapeHtml(cmd.response) + '</div>';
                    commandsDiv.appendChild(card);
                });

            } catch (error) {
                loading.style.display = 'none';
                showError(error.message);
            }
        }

        function showError(message) {
            const errorDiv = document.getElementById('error');
            errorDiv.innerHTML = '<div class="error">' + escapeHtml(message) + '</div>';
            document.getElementById('commands').innerHTML = '';
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        // Allow Enter key to trigger search
        document.getElementById('guildId').addEventListener('keypress', function(e) {
            if (e.key === 'Enter') loadCommands();
        });
        document.getElementById('searchQuery').addEventListener('keypress', function(e) {
            if (e.key === 'Enter') loadCommands();
        });
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
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
