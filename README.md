# nkmzbot

Discord bot with REST API for managing custom commands.

## 必要な環境変数

- DISCORD_TOKEN: ボットトークン
- DATABASE_URL: Postgres 接続文字列
- WEB_BIND: Web サーバのバインドアドレス (例: `0.0.0.0:3000`、省略時はこの値)
- DISCORD_CLIENT_ID: Discord OAuth2 のクライアント ID
- DISCORD_CLIENT_SECRET: Discord OAuth2 のクライアントシークレット
- DISCORD_REDIRECT_URI: OAuth2 コールバック URL (例: `http://localhost:3000/api/auth/callback`)
- JWT_SECRET: JWT 署名用のシークレット文字列 (ランダムな長い文字列推奨)

## 起動方法(ローカル)

- `.env` などで上記環境変数を設定
- `go run cmd/nkmzbot/main.go` で Bot と API サーバーの両方が起動します
- API は `http://localhost:3000/api` でアクセス可能

## 認証方法

すべてのコマンドデータの取得には認証が必要です。

### OAuth2 認証フロー

1. `/api/auth/login` にアクセスして認証URLを取得
2. DiscordのOAuth2認証を完了
3. `/api/auth/callback` でJWTトークンがHTTP-Onlyクッキーに保存される
4. 以降のAPIリクエストは自動的にクッキーから認証される

または、`Authorization: Bearer <token>` ヘッダーでJWTトークンを送信することも可能です。

## API エンドポイント

### 認証
- `GET /api/auth/login` - OAuth2 ログイン URL を取得
- `GET /api/auth/callback` - OAuth2 コールバック (JWT トークンをクッキーに保存)
- `POST /api/auth/logout` - ログアウト (クッキーをクリア)

### ギルド管理
- `GET /api/user/guilds` - ユーザーが参加しているギルド一覧を取得 (認証必要)

### コマンド管理 (すべて認証必要)
- `GET /api/guilds/{guild_id}/commands` - コマンド一覧を取得
  - クエリパラメータ: `q` (検索キーワード)
- `POST /api/guilds/{guild_id}/commands` - コマンドを追加
  - Body: `{"name": "command_name", "response": "response_text"}`
- `PUT /api/guilds/{guild_id}/commands/{name}` - コマンドを更新
  - Body: `{"response": "new_response_text"}`
- `DELETE /api/guilds/{guild_id}/commands/{name}` - コマンドを削除
- `POST /api/guilds/{guild_id}/commands/bulk-delete` - 複数コマンドを削除
  - Body: `{"names": ["command1", "command2"]}`

## Docker

Docker で動かす場合、`WEB_BIND=0.0.0.0:3000` を必ず指定し、ポートを公開してください。

```bash
# 例
WEB_BIND=0.0.0.0:3000 \
DISCORD_TOKEN=... \
DATABASE_URL=postgres://... \
DISCORD_CLIENT_ID=... \
DISCORD_CLIENT_SECRET=... \
DISCORD_REDIRECT_URI=http://localhost:3000/api/auth/callback \
JWT_SECRET=$(openssl rand -hex 32) \
go run cmd/nkmzbot/main.go
```

## ビルド

```bash
go build -o nkmzbot cmd/nkmzbot/main.go
./nkmzbot
```
