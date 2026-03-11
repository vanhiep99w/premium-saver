# Architecture

## High-Level Shape

The codebase is a small monolith with package-level separation by concern rather than domain layering.
The primary runtime path is:

1. CLI startup in [`main.go`](/home/hieptran/Desktop/premium-saver/main.go)
2. Optional auth store and SQLite initialization
3. Reverse proxy server creation in [`proxy/server.go`](/home/hieptran/Desktop/premium-saver/proxy/server.go)
4. Optional admin route registration through [`admin/handlers.go`](/home/hieptran/Desktop/premium-saver/admin/handlers.go)
5. `http.ListenAndServe` with one shared `ServeMux`

## Packages And Responsibilities

- [`config`](/home/hieptran/Desktop/premium-saver/config): constants, env lookup, filesystem paths
- [`auth`](/home/hieptran/Desktop/premium-saver/auth): GitHub OAuth device flow and Copilot token lifecycle
- [`proxy`](/home/hieptran/Desktop/premium-saver/proxy): reverse proxy, header injection, usage extraction, cost estimation
- [`db`](/home/hieptran/Desktop/premium-saver/db): schema setup and query layer for users and usage
- [`admin`](/home/hieptran/Desktop/premium-saver/admin): session auth, CSRF checks, HTML and JSON handlers
- [`web`](/home/hieptran/Desktop/premium-saver/web): embedded templates and static files

## Request Lifecycle

For public proxy traffic:

1. Request enters `ServeMux`
2. Unmatched paths fall into `handleProxy` in [`proxy/server.go`](/home/hieptran/Desktop/premium-saver/proxy/server.go)
3. If multi-user mode is active, API key is resolved to a local user in SQLite
4. Reverse proxy director rewrites host/path and injects Copilot headers
5. Proxy sends request to GitHub Copilot
6. Response passes through `ModifyResponse`
7. Streaming and JSON responses are wrapped for usage extraction
8. Usage records are written asynchronously through [`proxy/tracker.go`](/home/hieptran/Desktop/premium-saver/proxy/tracker.go)

## Admin Flow

1. Templates are parsed during startup in [`main.go`](/home/hieptran/Desktop/premium-saver/main.go)
2. Admin account is seeded from env vars in [`admin/handlers.go`](/home/hieptran/Desktop/premium-saver/admin/handlers.go)
3. Session state is kept in memory via [`admin/session.go`](/home/hieptran/Desktop/premium-saver/admin/session.go)
4. Page routes render embedded HTML templates
5. Browser JS calls JSON endpoints for create, update, delete, and chart data

## Persistence Model

- Auth tokens are file-backed, process-local state
- Admin sessions are memory-only and expire after one month
- Users and usage reports are persisted in SQLite

## Notable Architectural Traits

- No DI container, service interfaces, or middleware stack
- Most behavior is wired directly in constructors and handlers
- Good fit for a small binary, but coupling is relatively high
- Admin, tracking, and proxy concerns all meet inside [`proxy/server.go`](/home/hieptran/Desktop/premium-saver/proxy/server.go)
