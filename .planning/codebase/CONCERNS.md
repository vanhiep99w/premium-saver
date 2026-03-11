# Concerns

## Highest-Risk Areas

- No automated tests for auth, proxy rewriting, usage accounting, or admin security
- Hardcoded Copilot impersonation headers in [`config/config.go`](/home/hieptran/Desktop/premium-saver/config/config.go) may drift as upstream expectations change
- Usage tracking depends on response JSON shape and final SSE usage chunks in [`proxy/tracker.go`](/home/hieptran/Desktop/premium-saver/proxy/tracker.go)
- Sessions are in-memory only, so admin login state is lost on restart and does not support multi-instance deployment

## Security Concerns

- CORS is set to `*` for all proxy responses in [`proxy/server.go`](/home/hieptran/Desktop/premium-saver/proxy/server.go)
- Admin cookie setup in [`admin/session.go`](/home/hieptran/Desktop/premium-saver/admin/session.go) does not set the `Secure` flag
- API key extraction accepts raw `Authorization` header values as a fallback, which is permissive
- `auth.Store.Clear()` in [`auth/store.go`](/home/hieptran/Desktop/premium-saver/auth/store.go) returns an error if the auth file is already missing

## Performance Concerns

- `injectStreamOptions` reads and rewrites entire request bodies in memory
- `ParseNonStreamUsage` buffers entire JSON responses in memory before re-serving them
- Usage tracking drops records when the channel fills up
- Admin pages compute per-user stats with repeated DB queries rather than batched aggregation

## Maintainability Concerns

- [`proxy/server.go`](/home/hieptran/Desktop/premium-saver/proxy/server.go) combines routing, auth gating, proxy behavior, usage tracking hookup, and admin setup
- Schema migration is a single SQL blob with opportunistic `ALTER TABLE` instead of versioned migrations
- Error response contracts differ across handlers and modes

## Product And Operational Concerns

- README likely lags behind some recent in-flight changes shown in the dirty worktree
- Cost estimation in [`proxy/pricing.go`](/home/hieptran/Desktop/premium-saver/proxy/pricing.go) is approximate and can become stale quickly
- The project depends on undocumented or semi-internal Copilot behavior, so breakage risk is external

## Suggested Follow-Up

- Add high-value tests around proxy/auth/tracking first
- Separate admin and proxy concerns more cleanly over time
- Normalize JSON error payloads and logging
- Review cookie security, CORS posture, and auth file deletion behavior
