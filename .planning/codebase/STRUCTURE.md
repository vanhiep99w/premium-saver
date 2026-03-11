# Structure

## Top-Level Layout

- [`main.go`](/home/hieptran/Desktop/premium-saver/main.go): CLI entrypoint and bootstrap
- [`go.mod`](/home/hieptran/Desktop/premium-saver/go.mod): module definition
- [`install.sh`](/home/hieptran/Desktop/premium-saver/install.sh): release installer
- [`README.md`](/home/hieptran/Desktop/premium-saver/README.md): usage and deployment docs

## Go Packages

- [`auth`](/home/hieptran/Desktop/premium-saver/auth)
- `auth.go`: OAuth device flow, token refresh, login/logout/status
- `store.go`: local JSON token persistence

- [`config`](/home/hieptran/Desktop/premium-saver/config)
- `config.go`: constants, env vars, path helpers

- [`proxy`](/home/hieptran/Desktop/premium-saver/proxy)
- `server.go`: HTTP server, routes, reverse proxy behavior
- `headers.go`: request header mutation
- `stream_options.go`: request body injection for streaming usage
- `tracker.go`: usage extraction and async DB writes
- `pricing.go`: model pricing table and cost calculation

- [`db`](/home/hieptran/Desktop/premium-saver/db)
- `db.go`: database open, schema migration, cleanup job
- `users.go`: CRUD for users and API key hashing
- `usage.go`: usage insertions and reporting queries

- [`admin`](/home/hieptran/Desktop/premium-saver/admin)
- `handlers.go`: login, users, reports, JSON endpoints
- `session.go`: session and CSRF management

- [`web`](/home/hieptran/Desktop/premium-saver/web)
- `embed.go`: embeds HTML and static assets
- `templates/`: `layout.html`, `login.html`, `users.html`, `report.html`
- `static/`: `style.css`, `app.js`

## Supporting Material

- [`docs/superpowers/specs`](/home/hieptran/Desktop/premium-saver/docs/superpowers/specs): design/spec notes
- [`docs/superpowers/plans`](/home/hieptran/Desktop/premium-saver/docs/superpowers/plans): planning notes
- [`opencode-plugin`](/home/hieptran/Desktop/premium-saver/opencode-plugin): related integration docs and plugin file

## Naming Patterns

- Packages use short, direct names aligned with responsibility
- Files are named after a cohesive concern rather than type-per-file
- Handlers use `handleX` naming
- CLI functions use `cmdX` naming

## Current State Notes

- The repo has uncommitted changes in proxy, db, admin, and web files
- There are no nested submodules or build workspaces
- The layout is intentionally flat and easy to scan
