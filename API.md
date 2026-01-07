# nkmzbot API Documentation

## Overview

The nkmzbot API is a RESTful API that allows you to manage Discord bot commands through HTTP requests. All endpoints return JSON responses.

## Web Interface

The bot includes a web interface for viewing commands:
- `http://localhost:3000/` - Main page where you can enter a Guild ID
- `http://localhost:3000/guilds/{guild_id}` - View all commands for a specific guild

The web interface does not require authentication and is publicly accessible.

## Authentication

Most management endpoints require authentication. The API uses JWT tokens obtained through Discord OAuth2. Public read-only endpoints do not require authentication.

### Getting a Token

1. **Get the OAuth2 URL**
```bash
curl http://localhost:3000/api/auth/login
```

Response:
```json
{
  "auth_url": "https://discord.com/api/oauth2/authorize?...",
  "state": "random_state_string"
}
```

2. **Complete OAuth2 flow**
   - Direct the user to the `auth_url`
   - Discord will redirect back to your `DISCORD_REDIRECT_URI` with a code
   - The callback endpoint will return a JWT token

3. **Use the token**
   - Include the token in the `Authorization` header as `Bearer <token>`

## Endpoints

### Public Endpoints

#### GET /api/public/guilds/{guild_id}/commands
List all commands for a specific guild (no authentication required).

**Query Parameters:**
- `q` (optional): Search keyword to filter commands

**Response:**
```json
[
  {
    "guild_id": 123456789,
    "name": "hello",
    "response": "Hello, world!"
  }
]
```

**Example:**
```bash
curl "http://localhost:3000/api/public/guilds/123456789/commands?q=hello"
```

### Authentication

#### GET /api/auth/login
Get the Discord OAuth2 authorization URL.

**Response:**
```json
{
  "auth_url": "https://discord.com/api/oauth2/authorize?...",
  "state": "random_state"
}
```

#### GET /api/auth/callback
OAuth2 callback endpoint. Called by Discord after user authorization.

**Query Parameters:**
- `code`: Authorization code from Discord

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": "123456789",
  "username": "username"
}
```

#### POST /api/auth/logout
Logout endpoint (client should discard the JWT token).

**Response:**
```json
{
  "message": "logged out"
}
```

### Guilds

#### GET /api/user/guilds
Get list of guilds the authenticated user belongs to.

**Headers:**
- `Authorization: Bearer <token>`

**Response:**
```json
[
  {
    "id": "123456789",
    "name": "My Guild",
    "owner": false
  }
]
```

### Commands

#### GET /api/guilds/{guild_id}/commands
List all commands for a specific guild.

**Headers:**
- `Authorization: Bearer <token>`

**Query Parameters:**
- `q` (optional): Search keyword to filter commands

**Response:**
```json
[
  {
    "guild_id": 123456789,
    "name": "hello",
    "response": "Hello, world!"
  }
]
```

**Example:**
```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:3000/api/guilds/123456789/commands?q=hello"
```

#### POST /api/guilds/{guild_id}/commands
Add a new command.

**Headers:**
- `Authorization: Bearer <token>`
- `Content-Type: application/json`

**Body:**
```json
{
  "name": "hello",
  "response": "Hello, world!"
}
```

**Response:**
```json
{
  "message": "command added"
}
```

**Example:**
```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"hello","response":"Hello, world!"}' \
  http://localhost:3000/api/guilds/123456789/commands
```

#### PUT /api/guilds/{guild_id}/commands/{name}
Update an existing command.

**Headers:**
- `Authorization: Bearer <token>`
- `Content-Type: application/json`

**Body:**
```json
{
  "response": "New response text"
}
```

**Response:**
```json
{
  "message": "command updated"
}
```

**Example:**
```bash
curl -X PUT \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"response":"Updated response"}' \
  http://localhost:3000/api/guilds/123456789/commands/hello
```

#### DELETE /api/guilds/{guild_id}/commands/{name}
Delete a command.

**Headers:**
- `Authorization: Bearer <token>`

**Response:**
```json
{
  "message": "command deleted"
}
```

**Example:**
```bash
curl -X DELETE \
  -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:3000/api/guilds/123456789/commands/hello
```

#### POST /api/guilds/{guild_id}/commands/bulk-delete
Delete multiple commands at once.

**Headers:**
- `Authorization: Bearer <token>`
- `Content-Type: application/json`

**Body:**
```json
{
  "names": ["command1", "command2", "command3"]
}
```

**Response:**
```json
{
  "deleted": 3,
  "errors": []
}
```

If some deletions fail:
```json
{
  "deleted": 2,
  "errors": [
    "Failed to delete 'command3': command not found"
  ]
}
```

**Example:**
```bash
curl -X POST \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"names":["hello","goodbye"]}' \
  http://localhost:3000/api/guilds/123456789/commands/bulk-delete
```

## Error Responses

All error responses follow this format:

```json
{
  "error": "Error message"
}
```

Common HTTP status codes:
- `400 Bad Request` - Invalid request body or parameters
- `401 Unauthorized` - Missing or invalid authentication token
- `403 Forbidden` - User doesn't have access to the requested resource
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error

## CORS

The API supports CORS to allow requests from web browsers. By default, it allows all origins (`*`) for development purposes. For production, you should configure specific allowed origins in the code.

## Rate Limiting

Currently, there is no rate limiting implemented. Consider adding rate limiting for production use.
