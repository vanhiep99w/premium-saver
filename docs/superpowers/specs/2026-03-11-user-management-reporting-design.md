# User Management & Usage Reporting for Copilot Proxy

**Date:** 2026-03-11
**Status:** Approved

## Overview

Upgrade the existing copilot-proxy (a Go reverse proxy for GitHub Copilot API) with multi-user support, API key authentication, and per-user usage reporting. The proxy currently has zero external dependencies, no database, no multi-user support, and no incoming authentication.

## Goals

- Admin can add/remove users, each with a unique API key
- Track request count and token usage per user
- Per-user report page showing stats for 1h, 5h, 1 day, 1 month periods
- Auto-delete usage data older than 30 days

## Architecture

### Monolith approach

Single binary, single port. Admin UI and proxy share the same HTTP server.

Routing is explicit — the router checks paths in this order:

1. `/health` → health check handler
2. `/admin/*` → admin UI handlers
3. Everything else → reverse proxy to Copilot (with API key validation)

This replaces the current catch-all approach where every non-`/health` request goes to the proxy.

### Package structure

```
main.go              -- add admin setup + DB init
auth/                -- keep existing OAuth flow
config/              -- add admin password, DB path config
proxy/
  server.go          -- explicit routing (admin, health, proxy), API key validation
  headers.go         -- keep existing
  tracker.go         -- (new) stream parser + write usage_logs
db/
  db.go              -- SQLite init, migrations
  users.go           -- CRUD users + API key gen
  usage.go           -- query usage stats, cleanup job
admin/
  handlers.go        -- login, user management, report handlers
  session.go         -- cookie session management
web/
  templates/         -- Go html/template files (embed)
    layout.html
    login.html
    users.html
    report.html
  static/            -- CSS, JS (embed)
    style.css
    app.js
```

### New dependencies

- `modernc.org/sqlite` — pure Go SQLite driver (no CGO required)
- `github.com/google/uuid` — API key generation

### Configuration

| Env Variable | Default | Description |
|---|---|---|
| `ADMIN_USERNAME` | `admin` | Admin login username |
| `ADMIN_PASSWORD` | (required) | Admin login password. If not set, admin UI is disabled |
| `DB_PATH` | `~/.config/copilot-proxy/copilot-proxy.db` | SQLite database file path |

## Database Schema

```sql
PRAGMA journal_mode=WAL;

CREATE TABLE admin (
    id            INTEGER PRIMARY KEY CHECK (id = 1),
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL
);

CREATE TABLE users (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    api_key_hash TEXT NOT NULL UNIQUE,
    api_key_prefix TEXT NOT NULL,
    active      BOOLEAN DEFAULT 1,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE usage_logs (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model             TEXT,
    path              TEXT,
    prompt_tokens     INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    total_tokens      INTEGER DEFAULT 0,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_usage_logs_user_time ON usage_logs(user_id, created_at);
```

Key changes from initial draft:
- `PRAGMA journal_mode=WAL` for concurrent read/write support
- `admin.username` has UNIQUE constraint
- `users` table stores `api_key_hash` (SHA-256) instead of plaintext, plus `api_key_prefix` (first 11 chars, e.g. `sk-550e8400`) for display in admin UI
- `ON DELETE CASCADE` on usage_logs foreign key

## Routing

### Proxy routes (existing, modified)

```
GET    /health       -- health check (keep existing)
*      /*            -- proxy to Copilot (everything not matched above, with API key validation)
```

### Admin routes (new)

```
GET    /admin/login             -- login page
POST   /admin/login             -- submit login (throttled: per-IP, exponential backoff after 3 failures: 1s, 2s, 4s, max 30s)
POST   /admin/logout            -- logout
GET    /admin/                  -- redirect to /admin/users
GET    /admin/users             -- user management page (table view)
POST   /admin/users             -- create user (returns API key)
DELETE /admin/users/{id}        -- delete user
PATCH  /admin/users/{id}        -- update user, body: {"active": true/false}
GET    /admin/report/{id}       -- report page for a user
GET    /admin/api/report/{id}   -- JSON report data (for JS fetch)
```

Note: Uses Go 1.22+ `{id}` path parameter syntax (project uses Go 1.23.6).

## Request Flow (Proxy)

1. Client sends request with `Authorization: Bearer <user_api_key>`
2. Proxy extracts the API key from the Authorization header
3. Compute SHA-256 hash of the API key, lookup `api_key_hash` in SQLite → find user
4. If not found → `401 {"error": "invalid api key"}`
5. If user inactive → `403 {"error": "user is inactive"}`
6. Replace the Authorization header with the Copilot token (existing `InjectHeaders` behavior — the client's API key is consumed and replaced)
7. Forward request to Copilot using shared OAuth token
8. Parse streaming response via `ModifyResponse`: swap `resp.Body` with a pipe-through reader that intercepts SSE chunks and extracts `usage` field from the final chunk
9. Send usage data to a buffered channel; a single writer goroutine drains the channel and batch-writes to SQLite
10. Client receives streamed response in real-time (not blocked by usage tracking)

## Authentication

### GitHub Copilot

All users share the admin's GitHub OAuth token. Individual GitHub account linking is out of scope for this version.

### Admin UI

- Single admin account
- Credentials configured via environment variables: `ADMIN_USERNAME` (default: `admin`) and `ADMIN_PASSWORD` (required, no default)
- If `ADMIN_PASSWORD` is not set, the server prints a warning and disables admin UI
- On first run, if no admin row exists in DB, create one with bcrypt-hashed password
- On subsequent runs, if env var password differs from stored hash, update the hash (allows password changes via env var)
- Login creates a 32-byte random session token stored in HttpOnly cookie (SameSite=Strict)
- Session stored in-memory (`map[string]sessionInfo` with expiry timestamp), expires after 1 month
- Sessions do not survive server restarts (acceptable — admin re-logs in)
- Expired sessions are lazily evicted on access
- All `/admin/*` routes (except `/admin/login`) require valid session
- Admin routes do NOT set `Access-Control-Allow-Origin` headers (CORS is only for proxy routes)
- Admin POST/DELETE/PATCH routes require a CSRF token (generated per session, rendered into HTML pages as a `<meta name="csrf-token">` tag, sent by JavaScript via `X-CSRF-Token` header for fetch requests or hidden form field for form submissions)

### API Key format

- Generated as `sk-` + UUID v4 (e.g., `sk-550e8400-e29b-41d4-a716-446655440000`)
- Stored as SHA-256 hash in SQLite (`api_key_hash` column)
- `api_key_prefix` stores first 11 chars of the full key for display (e.g., `sk-550e8400`)
- Full API key is shown once when user is created; cannot be retrieved afterward

## UI Design

### Tech stack

Go `html/template` + vanilla JavaScript. Templates and static files embedded into binary via `embed.FS`. Dark theme.

### Users page (Table View)

- Header with "Users" tab active
- Table columns: Name, API Key (prefix only), Status (active/inactive), Requests (24h), Actions (report, delete)
- "Add User" button opens a form/modal to enter name
- After creation, show the full API key once with copy button
- Toggle active/inactive via PATCH request with body `{"active": true/false}`
- Delete with confirmation dialog

### Report page (Cards + Recent Log)

- Header: user name + status + back link
- Request Count section: 4 cards showing count for 1h, 5h, 1 day, 1 month
- Token Usage section: 4 cards showing total tokens with prompt/completion breakdown (P:xxx C:xxx)
- Recent Requests section: table with time, path, model, tokens for last 50 requests

## Token Tracking

Parse the `usage` field from streaming (SSE) responses:

- In `ModifyResponse`, wrap `resp.Body` with a custom `io.ReadCloser` that tees data through while scanning for the `usage` JSON object in SSE chunks
- The final data chunk before `data: [DONE]` typically contains `usage` object with `prompt_tokens`, `completion_tokens`, `total_tokens`
- For non-streaming responses (no `text/event-stream` content type), read and re-serve the body, parsing the JSON `usage` field
- Token tracking is best-effort: if parsing fails, log the error but do not block the response
- Usage data is sent to a buffered channel (capacity 1000). A single writer goroutine drains the channel and writes to SQLite. If the channel is full, the usage record is dropped with a log warning. If SQLite writes fail persistently, the writer backs off (1s, 2s, 4s, max 30s) to avoid log flooding.

## Data Cleanup

- A goroutine runs every hour: `DELETE FROM usage_logs WHERE created_at < datetime('now', '-30 days')`
- When deleting a user, cascade delete their usage_logs (via `ON DELETE CASCADE`)

## Error Handling

- Invalid API key → `401 {"error": "invalid api key"}`
- Inactive user → `403 {"error": "user is inactive"}`
- Copilot token expired → auto-refresh (existing logic)
- SQLite write failure → log error, do not block proxy response
- Admin login failure → show error on login page, per-IP exponential backoff after 3 consecutive failures (1s, 2s, 4s, max 30s; state in-memory, auto-expires after 15 minutes)

## Out of Scope (YAGNI)

- Rate limiting (except admin login throttling)
- User self-service (change API key, view own report)
- Export CSV/PDF
- Charts/graphs (text stats only)
- Per-user GitHub account linking
- Multiple admin accounts
- Graceful shutdown (existing codebase doesn't have it either)
- Migration path from single-user mode (breaking change, documented in release notes)
