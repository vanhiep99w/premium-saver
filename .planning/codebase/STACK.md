# Stack

## Overview

This project is a small Go monolith that ships as a single binary named `copilot-proxy`.
It combines a CLI, an HTTP reverse proxy, local token storage, optional SQLite-backed user management, and a server-rendered admin UI.

## Core Languages And Runtime

- Go `1.23.6` from [`go.mod`](/home/hieptran/Desktop/premium-saver/go.mod)
- Standard library-heavy implementation using `net/http`, `httputil`, `html/template`, `embed`, `database/sql`
- Single-process runtime started from [`main.go`](/home/hieptran/Desktop/premium-saver/main.go)

## Direct Dependencies

- `modernc.org/sqlite` for embedded SQLite without CGO
- `golang.org/x/crypto` for bcrypt password hashing
- `github.com/google/uuid` for generating API keys

## Storage

- OAuth and Copilot tokens stored in a local JSON file via [`auth/store.go`](/home/hieptran/Desktop/premium-saver/auth/store.go)
- Multi-user and usage tracking data stored in SQLite via [`db/db.go`](/home/hieptran/Desktop/premium-saver/db/db.go)
- SQLite is opened in WAL mode with foreign keys enabled

## Web Layer

- HTTP server uses `http.ServeMux` in [`proxy/server.go`](/home/hieptran/Desktop/premium-saver/proxy/server.go)
- Admin pages are server-rendered HTML templates embedded with [`web/embed.go`](/home/hieptran/Desktop/premium-saver/web/embed.go)
- Admin frontend JavaScript is plain browser JS in [`web/static/app.js`](/home/hieptran/Desktop/premium-saver/web/static/app.js)
- Charts rely on Chart.js loaded from CDN in [`web/templates/layout.html`](/home/hieptran/Desktop/premium-saver/web/templates/layout.html)

## Configuration Surface

- CLI command dispatch and `-p` port flag in [`main.go`](/home/hieptran/Desktop/premium-saver/main.go)
- Environment variables from [`config/config.go`](/home/hieptran/Desktop/premium-saver/config/config.go):
- `ADMIN_PASSWORD`
- `ADMIN_USERNAME`
- `DB_PATH`

## Build And Distribution

- Local build: `go build -o copilot-proxy .`
- Install script: [`install.sh`](/home/hieptran/Desktop/premium-saver/install.sh)
- Distribution model is binary download from GitHub Releases

## Embedded Assets

- HTML templates in [`web/templates`](/home/hieptran/Desktop/premium-saver/web/templates)
- Static CSS and JS in [`web/static`](/home/hieptran/Desktop/premium-saver/web/static)

## Operational Modes

- Single-user mode when `ADMIN_PASSWORD` is unset
- Multi-user mode with admin UI and usage tracking when `ADMIN_PASSWORD` is set
