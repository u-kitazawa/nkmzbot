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

func (a *API) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>nkmzbot - ログイン</title>
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
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .login-container {
            background: white;
            border-radius: 20px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            padding: 60px 40px;
            max-width: 480px;
            width: 100%;
            text-align: center;
        }
        .logo {
            font-size: 4rem;
            margin-bottom: 20px;
        }
        h1 {
            color: #333;
            font-size: 2rem;
            margin-bottom: 10px;
        }
        .subtitle {
            color: #666;
            font-size: 1rem;
            margin-bottom: 40px;
        }
        .login-btn {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 12px;
            background: #5865F2;
            color: white;
            border: none;
            border-radius: 12px;
            padding: 16px 32px;
            font-size: 1.1rem;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s;
            text-decoration: none;
            width: 100%;
        }
        .login-btn:hover {
            background: #4752C4;
            transform: translateY(-2px);
            box-shadow: 0 8px 20px rgba(88, 101, 242, 0.4);
        }
        .login-btn:active {
            transform: translateY(0);
        }
        .discord-icon {
            width: 24px;
            height: 24px;
        }
        .info-box {
            background: #f5f5f5;
            border-radius: 12px;
            padding: 20px;
            margin-top: 30px;
            text-align: left;
        }
        .info-box h3 {
            color: #333;
            font-size: 1rem;
            margin-bottom: 10px;
        }
        .info-box ul {
            color: #666;
            font-size: 0.9rem;
            line-height: 1.8;
            padding-left: 20px;
        }
        .error {
            background: #fee;
            color: #c33;
            padding: 15px;
            border-radius: 8px;
            margin-bottom: 20px;
        }
        .success {
            background: #efe;
            color: #3c3;
            padding: 15px;
            border-radius: 8px;
            margin-bottom: 20px;
        }
        .token-display {
            background: #f5f5f5;
            border: 2px solid #ddd;
            border-radius: 8px;
            padding: 15px;
            margin-top: 20px;
            word-break: break-all;
            font-family: monospace;
            font-size: 0.85rem;
            text-align: left;
        }
        .copy-btn {
            background: #667eea;
            color: white;
            border: none;
            border-radius: 6px;
            padding: 8px 16px;
            margin-top: 10px;
            cursor: pointer;
            font-size: 0.9rem;
        }
        .copy-btn:hover {
            background: #5568d3;
        }
        .home-link {
            display: inline-block;
            margin-top: 20px;
            color: #667eea;
            text-decoration: none;
            font-size: 0.95rem;
        }
        .home-link:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="logo">🤖</div>
        <h1>nkmzbot</h1>
        <p class="subtitle">Discord Bot 管理システム</p>
        
        <div id="message"></div>
        <div id="tokenDisplay"></div>
        
        <button id="loginBtn" class="login-btn" onclick="login()">
            <svg class="discord-icon" viewBox="0 0 24 24" fill="currentColor">
                <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515a.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0a12.64 12.64 0 0 0-.617-1.25a.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057a19.9 19.9 0 0 0 5.993 3.03a.078.078 0 0 0 .084-.028a14.09 14.09 0 0 0 1.226-1.994a.076.076 0 0 0-.041-.106a13.107 13.107 0 0 1-1.872-.892a.077.077 0 0 1-.008-.128a10.2 10.2 0 0 0 .372-.292a.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127a12.299 12.299 0 0 1-1.873.892a.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028a19.839 19.839 0 0 0 6.002-3.03a.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419c0-1.333.956-2.419 2.157-2.419c1.21 0 2.176 1.096 2.157 2.42c0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419c0-1.333.955-2.419 2.157-2.419c1.21 0 2.176 1.096 2.157 2.42c0 1.333-.946 2.418-2.157 2.418z"/>
            </svg>
            Discord でログイン
        </button>

        <div class="info-box">
            <h3>📌 ログインについて</h3>
            <ul>
                <li>Discord アカウントでログイン</li>
                <li>ギルド管理権限が必要です</li>
                <li>コマンドの追加・編集・削除が可能</li>
                <li>トークンは24時間有効です</li>
            </ul>
        </div>

        <a href="/" class="home-link">← コマンド一覧に戻る</a>
    </div>

    <script>
        async function login() {
            try {
                const response = await fetch('/api/auth/login');
                const data = await response.json();
                
                if (data.auth_url) {
                    window.location.href = data.auth_url;
                } else {
                    showMessage('error', 'ログインURLの取得に失敗しました');
                }
            } catch (error) {
                showMessage('error', 'エラーが発生しました: ' + error.message);
            }
        }

        function showMessage(type, text) {
            const messageDiv = document.getElementById('message');
            messageDiv.className = type;
            messageDiv.textContent = text;
        }

        function copyToken() {
            const token = window.location.hash.substring(7); // Remove #token=
            navigator.clipboard.writeText(token).then(() => {
                alert('トークンをコピーしました！');
            }).catch(() => {
                alert('トークンのコピーに失敗しました。手動でコピーしてください。');
            });
        }

        // Check for success/error in URL
        window.addEventListener('DOMContentLoaded', () => {
            const urlParams = new URLSearchParams(window.location.search);
            const error = urlParams.get('error');
            const success = urlParams.get('success');
            
            if (error) {
                const errorMessages = {
                    'token_exchange_failed': 'トークンの交換に失敗しました',
                    'failed_to_get_user': 'ユーザー情報の取得に失敗しました',
                    'failed_to_create_token': 'トークンの作成に失敗しました'
                };
                showMessage('error', errorMessages[error] || 'エラーが発生しました');
            } else if (success && window.location.hash.startsWith('#token=')) {
                const token = window.location.hash.substring(7);
                
                showMessage('success', 'ログインに成功しました！');
                
                const tokenDisplay = document.getElementById('tokenDisplay');
                tokenDisplay.innerHTML = 
                    '<div class="token-display">' +
                    '<strong>認証トークン:</strong><br>' +
                    token +
                    '</div>' +
                    '<button class="copy-btn" onclick="copyToken()">トークンをコピー</button>' +
                    '<p style="margin-top: 15px; color: #666; font-size: 0.9rem;">' +
                    'このトークンを Authorization ヘッダーに「Bearer ' + token.substring(0, 20) + '...」として設定してください。' +
                    '</p>' +
                    '<p style="margin-top: 10px; color: #f44336; font-size: 0.85rem;">' +
                    '⚠️ セキュリティ上の注意: このトークンは他者に共有しないでください。トークンは24時間で期限切れになります。' +
                    '</p>';
                
                document.getElementById('loginBtn').style.display = 'none';
                
                // Clear the token from URL after displaying (for security)
                setTimeout(() => {
                    history.replaceState(null, '', '/login?success=true');
                }, 1000);
            }
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
