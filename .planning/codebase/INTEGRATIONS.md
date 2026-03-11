# Integrations

## Upstream Services

The proxy is tightly coupled to GitHub and GitHub Copilot endpoints defined in [`config/config.go`](/home/hieptran/Desktop/premium-saver/config/config.go).

- GitHub device auth: `https://github.com/login/device/code`
- GitHub OAuth token exchange: `https://github.com/login/oauth/access_token`
- GitHub Copilot token endpoint: `https://api.github.com/copilot_internal/v2/token`
- GitHub Copilot API base: `https://api.githubcopilot.com`

## Authentication Flow

- Device flow starts in [`auth/auth.go`](/home/hieptran/Desktop/premium-saver/auth/auth.go)
- OAuth token is persisted to disk in [`auth/store.go`](/home/hieptran/Desktop/premium-saver/auth/store.go)
- Short-lived Copilot API tokens are refreshed on demand before proxying requests
- Proxy requests overwrite incoming credentials with the Copilot token in [`proxy/headers.go`](/home/hieptran/Desktop/premium-saver/proxy/headers.go)

## Local Database

- SQLite database path is resolved by [`config/config.go`](/home/hieptran/Desktop/premium-saver/config/config.go)
- Schema and cleanup job live in [`db/db.go`](/home/hieptran/Desktop/premium-saver/db/db.go)
- User records and hashed API keys live in [`db/users.go`](/home/hieptran/Desktop/premium-saver/db/users.go)
- Usage aggregations live in [`db/usage.go`](/home/hieptran/Desktop/premium-saver/db/usage.go)

## HTTP Clients

- The only explicit outbound HTTP client is in [`auth/auth.go`](/home/hieptran/Desktop/premium-saver/auth/auth.go)
- The reverse proxy in [`proxy/server.go`](/home/hieptran/Desktop/premium-saver/proxy/server.go) forwards all OpenAI-compatible traffic to Copilot

## Browser Dependencies

- Chart rendering depends on Chart.js CDN in [`web/templates/layout.html`](/home/hieptran/Desktop/premium-saver/web/templates/layout.html)
- The admin UI otherwise avoids npm or frontend build tooling

## Route Surface

Public endpoints:

- `GET /health`
- `/*` catch-all proxy

Admin endpoints, only when admin mode is enabled:

- `GET /admin/login`
- `POST /admin/login`
- `POST /admin/logout`
- `GET /admin/users`
- `POST /admin/users`
- `DELETE /admin/users/{id}`
- `PATCH /admin/users/{id}`
- `GET /admin/report/{id}`
- `GET /admin/api/report/{id}`
- `GET /admin/api/report/{id}/chart-data`

## Request Mutations

- `/v1/...` paths are rewritten to upstream paths in [`proxy/server.go`](/home/hieptran/Desktop/premium-saver/proxy/server.go)
- Streaming POST payloads are modified to add `stream_options.include_usage=true` in [`proxy/stream_options.go`](/home/hieptran/Desktop/premium-saver/proxy/stream_options.go)
- Headers are forced to impersonate VS Code Copilot Chat and set `X-Initiator: agent`
