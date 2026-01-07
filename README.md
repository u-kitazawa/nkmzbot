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
- Web インターフェース: `http://localhost:3000/guilds/{guild_id}` でコマンド一覧を表示

## Web インターフェース

コマンドの内容を Web ブラウザで表示できます:
- `http://localhost:3000/` - トップページ（Guild ID を入力してコマンド一覧を表示）
- `http://localhost:3000/guilds/{guild_id}` - 特定のギルドのコマンド一覧を表示
- `http://localhost:3000/login` - 認証ログインページ（Discord OAuth2）

コマンドの閲覧は認証不要で誰でも可能です。
コマンドの追加・編集・削除を行う場合は、ログインページから Discord アカウントで認証してください。

## API エンドポイント

### 認証
- `GET /api/auth/login` - OAuth2 ログイン URL を取得
- `GET /api/auth/callback` - OAuth2 コールバック (JWT トークンを返す)
- `POST /api/auth/logout` - ログアウト

### 公開エンドポイント
- `GET /api/public/guilds/{guild_id}/commands` - コマンド一覧を取得（認証不要）
  - クエリパラメータ: `q` (検索キーワード)

### ギルド管理
- `GET /api/user/guilds` - ユーザーが参加しているギルド一覧を取得 (認証必要)

### コマンド管理
- `GET /api/guilds/{guild_id}/commands` - コマンド一覧を取得 (認証必要)
  - クエリパラメータ: `q` (検索キーワード)
- `POST /api/guilds/{guild_id}/commands` - コマンドを追加 (認証必要)
  - Body: `{"name": "command_name", "response": "response_text"}`
- `PUT /api/guilds/{guild_id}/commands/{name}` - コマンドを更新 (認証必要)
  - Body: `{"response": "new_response_text"}`
- `DELETE /api/guilds/{guild_id}/commands/{name}` - コマンドを削除 (認証必要)
- `POST /api/guilds/{guild_id}/commands/bulk-delete` - 複数コマンドを削除 (認証必要)
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
