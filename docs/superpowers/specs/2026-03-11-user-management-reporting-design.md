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

```
/health        → health check (existing)
/admin/*       → admin UI (new)
/v1/*          → proxy to Copilot (existing, add API key validation)
```

### Package structure

```
main.go              -- add admin setup + DB init
auth/                -- keep existing OAuth flow
config/              -- add admin password, DB path config
proxy/
  server.go          -- add routing for /admin/*, API key validation
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

## Database Schema

```sql
CREATE TABLE admin (
    id            INTEGER PRIMARY KEY,
    username      TEXT NOT NULL,
    password_hash TEXT NOT NULL
);

CREATE TABLE users (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    api_key     TEXT NOT NULL UNIQUE,
    active      BOOLEAN DEFAULT 1,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE usage_logs (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id           INTEGER NOT NULL REFERENCES users(id),
    model             TEXT,
    path              TEXT,
    prompt_tokens     INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    total_tokens      INTEGER DEFAULT 0,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_usage_logs_user_time ON usage_logs(user_id, created_at);
```

## Routing

### Proxy routes (existing, modified)

```
GET    /health       -- health check (keep existing)
*      /v1/*         -- proxy to Copilot (add API key validation)
```

### Admin routes (new)

```
GET    /admin/login             -- login page
POST   /admin/login             -- submit login
POST   /admin/logout            -- logout
GET    /admin/                  -- redirect to /admin/users
GET    /admin/users             -- user management page (table view)
POST   /admin/users             -- create user (returns API key)
DELETE /admin/users/:id         -- delete user
PATCH  /admin/users/:id         -- toggle active/inactive
GET    /admin/report/:id        -- report page for a user
GET    /admin/api/report/:id    -- JSON report data (for JS fetch)
```

## Request Flow (Proxy)

1. Client sends request with `Authorization: Bearer <user_api_key>`
2. Proxy looks up `api_key` in SQLite → find user
3. If not found → `401 {"error": "invalid api key"}`
4. If user inactive → `403 {"error": "user is inactive"}`
5. Forward request to Copilot using shared OAuth token (existing flow)
6. Parse streaming response: pipe-through reader intercepts each SSE chunk, extracts `usage` field from final chunk
7. Write `usage_logs` record asynchronously (do not block response)
8. Client receives streamed response in real-time

## Authentication

### GitHub Copilot

All users share the admin's GitHub OAuth token. Individual GitHub account linking is out of scope for this version.

### Admin UI

- Single admin account with username/password set via environment variables or config
- Password stored as bcrypt hash in SQLite `admin` table
- On first run, if no admin exists, create one from config values
- Login creates a random session token stored in HttpOnly cookie
- Session stored in-memory (`map[string]time.Time`), expires after 1 month
- All `/admin/*` routes (except `/admin/login`) require valid session

### API Key format

- Generated as `sk-` + UUID v4 (e.g., `sk-550e8400-e29b-41d4-a716-446655440000`)
- Stored as plaintext in SQLite for fast lookup on every proxy request
- Shown once when user is created; admin can see it in the users table

## UI Design

### Tech stack

Go `html/template` + vanilla JavaScript. Templates and static files embedded into binary via `embed.FS`. Dark theme.

### Users page (Table View + Tabs)

- Navigation tabs: Users | (future tabs)
- Table columns: Name, API Key (truncated), Status (active/inactive), Requests (24h), Actions (report, delete)
- "Add User" button opens a form/modal to enter name
- After creation, show the full API key once with copy button
- Toggle active/inactive via PATCH request
- Delete with confirmation

### Report page (Cards + Recent Log)

- Header: user name + status + back link
- Request Count section: 4 cards showing count for 1h, 5h, 1 day, 1 month
- Token Usage section: 4 cards showing total tokens with prompt/completion breakdown (P:xxx C:xxx)
- Recent Requests section: table with time, path, model, tokens for last N requests

## Token Tracking

Parse the `usage` field from streaming (SSE) responses:

- Wrap the response body with a custom `io.Reader` that reads each SSE chunk
- The final chunk (or `data: [DONE]` predecessor) typically contains `usage` object with `prompt_tokens`, `completion_tokens`, `total_tokens`
- For non-streaming responses, parse the JSON body directly
- Token tracking is best-effort: if parsing fails, log the error but do not block the response

## Data Cleanup

- A goroutine runs every hour: `DELETE FROM usage_logs WHERE created_at < datetime('now', '-30 days')`
- When deleting a user, cascade delete their usage_logs

## Error Handling

- Invalid API key → `401 {"error": "invalid api key"}`
- Inactive user → `403 {"error": "user is inactive"}`
- Copilot token expired → auto-refresh (existing logic)
- SQLite write failure → log error, do not block proxy response
- Admin login failure → show error on login page

## Out of Scope (YAGNI)

- Rate limiting
- User self-service (change API key, view own report)
- Export CSV/PDF
- Charts/graphs (text stats only)
- Per-user GitHub account linking
- Multiple admin accounts
