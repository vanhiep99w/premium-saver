# Cost Tracking, Stream Options Fix & UI Charts

## Overview

Add cost estimation per request, fix streaming token tracking, and add visual charts to the admin dashboard.

## Feature 1: Cost Calculation

### Pricing Model
- Estimated cost based on public API pricing for each model provider
- Stored per-request in `usage_logs.estimated_cost` (REAL)
- Aggregated in stats queries alongside token counts

### Pricing Map (`proxy/pricing.go`)
- Map of model name prefix -> {input_price_per_mtok, output_price_per_mtok}
- Prefix matching: "claude-3.5-sonnet" matches "claude-3.5-sonnet-20241022"
- Default fallback pricing for unknown models
- `CalculateCost(model, promptTokens, completionTokens) float64`

### DB Changes
- Add `estimated_cost REAL DEFAULT 0` column to `usage_logs` (migration in `db.go`)
- Add `EstimatedCost float64` to `UsageRecord`
- Update `InsertUsage` to write `estimated_cost`
- Update `UsageStats` to include `EstimatedCost float64`
- Update `GetUsageStats` to `SUM(estimated_cost)`
- Update `RecentRequest` to include `EstimatedCost float64`

### Integration Points
- `tracker.go`: `emitUsage()` calls `CalculateCost()` before tracking
- `tracker.go`: `ParseNonStreamUsage()` calls `CalculateCost()` before tracking

## Feature 2: Stream Options Injection

### Approach
- Re-apply logic from commit 586b21a with improvements
- New file `proxy/stream_options.go` for clean separation
- Inject `stream_options: {include_usage: true}` into POST request body when `stream: true`
- Only inject if `stream_options` not already present
- Update Content-Length header after body modification
- Test locally with multiple models to verify compatibility

### Safeguards
- If Copilot API rejects the field for certain models, wrap in error handling
- Log when injection occurs for debugging

## Feature 3: Admin UI Charts

### New API Endpoints
- `GET /admin/api/report/{id}/chart-data` returns:
  - `hourly_usage`: last 24h token usage grouped by hour
  - `model_breakdown`: cost and token breakdown by model (last 30d)

### New DB Queries
- `GetHourlyUsage(userID int, hours int)` -> [{hour, prompt_tokens, completion_tokens, cost}]
- `GetModelBreakdown(userID int, days int)` -> [{model, prompt_tokens, completion_tokens, total_tokens, cost, request_count}]

### UI Changes
- Add Chart.js via CDN to `layout.html`
- Report page gets 3 new sections:
  1. Estimated Cost stats cards (same style as token cards)
  2. Token usage bar chart (hourly, last 24h)
  3. Cost by model doughnut chart
- Charts render via JS fetching `/admin/api/report/{id}/chart-data`
- Cost column added to recent requests table

### Chart.js Config
- Dark theme matching existing CSS variables
- Responsive, no animation on load for speed
- Tooltips with formatted values
