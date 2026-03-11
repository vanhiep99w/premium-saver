# Conventions

## General Style

- Standard Go formatting and package layout
- Small files grouped by concern
- Minimal abstraction layers and mostly direct function calls
- Comments are used sparingly and usually explain intent, not syntax

## Naming

- Exported types use idiomatic Go naming such as `Authenticator`, `Store`, `Server`, `UsageTracker`
- Handlers use `handleX`
- Constructors use `NewX`
- Helper functions are short and file-local where possible

## Error Handling

- CLI startup paths print a human-readable error to stderr and exit
- Library-like functions usually return `error` without wrapping UI behavior
- HTTP handlers frequently use `http.Error` or a small JSON helper from [`admin/handlers.go`](/home/hieptran/Desktop/premium-saver/admin/handlers.go)
- Some lower-level functions wrap errors with `%w`, especially around external IO

## State Management

- Shared mutable state is protected with mutexes in [`auth/store.go`](/home/hieptran/Desktop/premium-saver/auth/store.go) and [`admin/session.go`](/home/hieptran/Desktop/premium-saver/admin/session.go)
- Usage writes are offloaded onto a buffered channel in [`proxy/tracker.go`](/home/hieptran/Desktop/premium-saver/proxy/tracker.go)

## HTTP Conventions

- Uses Go 1.22 style method-aware route patterns such as `GET /admin/users`
- Admin routes expect CSRF tokens for state-changing methods
- API key auth is only enforced when SQLite mode is enabled

## Template And Frontend Conventions

- HTML is server-rendered with `html/template`
- Frontend behavior is plain JavaScript without framework state management
- CSS and JS are embedded into the binary through `embed.FS`

## Inconsistencies To Note

- Error response shapes are not fully consistent between proxy and admin endpoints
- Logging style mixes plain text warnings with structured-looking prefixes
- Some recently added files such as [`proxy/pricing.go`](/home/hieptran/Desktop/premium-saver/proxy/pricing.go) suggest the codebase is evolving without a strong central interface boundary
