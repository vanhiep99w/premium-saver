# Testing

## Current State

No Go test files were found in this repository.
There is no visible unit, integration, or end-to-end test suite under the current tree.

## What Exists Instead

- Behavior is primarily documented through [`README.md`](/home/hieptran/Desktop/premium-saver/README.md)
- There are planning and design notes under [`docs/superpowers`](/home/hieptran/Desktop/premium-saver/docs/superpowers)
- The project appears to rely on manual verification for CLI, proxy, and admin flows

## Areas That Need Tests First

- OAuth and Copilot token refresh behavior in [`auth/auth.go`](/home/hieptran/Desktop/premium-saver/auth/auth.go)
- Path rewriting and header injection in [`proxy/server.go`](/home/hieptran/Desktop/premium-saver/proxy/server.go) and [`proxy/headers.go`](/home/hieptran/Desktop/premium-saver/proxy/headers.go)
- Streaming usage extraction in [`proxy/tracker.go`](/home/hieptran/Desktop/premium-saver/proxy/tracker.go)
- Request body mutation in [`proxy/stream_options.go`](/home/hieptran/Desktop/premium-saver/proxy/stream_options.go)
- Session and CSRF behavior in [`admin/session.go`](/home/hieptran/Desktop/premium-saver/admin/session.go)
- SQLite migrations and reporting queries in [`db/db.go`](/home/hieptran/Desktop/premium-saver/db/db.go) and [`db/usage.go`](/home/hieptran/Desktop/premium-saver/db/usage.go)

## Suitable Test Strategy

- Table-driven unit tests for pure helpers and query logic
- `httptest`-based handler tests for admin and proxy routes
- Mock upstream server for reverse proxy and token refresh flows
- Temporary SQLite databases for integration-style DB tests

## Main Risk From Missing Tests

- Regressions in auth, billing tracking, and admin state can ship unnoticed
- Streaming behavior is especially fragile because it depends on exact upstream response shape
- Current uncommitted changes are modifying the most risk-sensitive areas without automated safety nets
