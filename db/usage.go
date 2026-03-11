package db

import "time"

// UsageRecord represents a single API request's usage data.
type UsageRecord struct {
	UserID           int
	Model            string
	Path             string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	EstimatedCost    float64
}

// UsageStats holds aggregated usage data for a time period.
type UsageStats struct {
	Period           string
	RequestCount     int
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	EstimatedCost    float64
}

// RecentRequest represents a single recent request for display.
type RecentRequest struct {
	Time             time.Time
	Path             string
	Model            string
	TotalTokens      int
	EstimatedCost    float64
}

// HourlyUsage holds usage data aggregated by hour.
type HourlyUsage struct {
	Hour             string  `json:"hour"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
}

// ModelBreakdown holds usage data aggregated by model.
type ModelBreakdown struct {
	Model            string  `json:"model"`
	RequestCount     int     `json:"request_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
}

// InsertUsage inserts a usage log record.
func (db *DB) InsertUsage(r UsageRecord) error {
	_, err := db.conn.Exec(
		`INSERT INTO usage_logs (user_id, model, path, prompt_tokens, completion_tokens, total_tokens, estimated_cost)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.UserID, r.Model, r.Path, r.PromptTokens, r.CompletionTokens, r.TotalTokens, r.EstimatedCost,
	)
	return err
}

// GetUsageStats returns aggregated usage stats for a user across multiple time periods.
func (db *DB) GetUsageStats(userID int) ([]UsageStats, error) {
	periods := []struct {
		name   string
		sqlArg string
	}{
		{"1h", "-1 hours"},
		{"5h", "-5 hours"},
		{"1d", "-1 days"},
		{"30d", "-30 days"},
	}

	var stats []UsageStats
	for _, p := range periods {
		var s UsageStats
		s.Period = p.name
		err := db.conn.QueryRow(
			`SELECT COUNT(*), COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0), COALESCE(SUM(total_tokens), 0), COALESCE(SUM(estimated_cost), 0)
			 FROM usage_logs WHERE user_id = ? AND created_at >= datetime('now', ?)`,
			userID, p.sqlArg,
		).Scan(&s.RequestCount, &s.PromptTokens, &s.CompletionTokens, &s.TotalTokens, &s.EstimatedCost)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// GetRecentRequests returns the last N requests for a user.
func (db *DB) GetRecentRequests(userID int, limit int) ([]RecentRequest, error) {
	rows, err := db.conn.Query(
		`SELECT created_at, path, model, total_tokens, estimated_cost FROM usage_logs
		 WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []RecentRequest
	for rows.Next() {
		var r RecentRequest
		if err := rows.Scan(&r.Time, &r.Path, &r.Model, &r.TotalTokens, &r.EstimatedCost); err != nil {
			return nil, err
		}
		reqs = append(reqs, r)
	}
	return reqs, rows.Err()
}

// GetTotalTokens returns the all-time total token usage for a user.
func (db *DB) GetTotalTokens(userID int) (int64, error) {
	var total int64
	err := db.conn.QueryRow(
		"SELECT COALESCE(SUM(total_tokens), 0) FROM usage_logs WHERE user_id = ?",
		userID,
	).Scan(&total)
	return total, err
}

// GetRequestCount24h returns the request count in the last 24 hours for a user.
func (db *DB) GetRequestCount24h(userID int) (int, error) {
	var count int
	err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM usage_logs WHERE user_id = ? AND created_at >= datetime('now', '-1 day')",
		userID,
	).Scan(&count)
	return count, err
}

// GetHourlyUsage returns token usage grouped by hour for the last N hours.
func (db *DB) GetHourlyUsage(userID int, hours int) ([]HourlyUsage, error) {
	rows, err := db.conn.Query(
		`SELECT strftime('%Y-%m-%dT%H:00:00Z', created_at) as hour,
		        COALESCE(SUM(prompt_tokens), 0),
		        COALESCE(SUM(completion_tokens), 0),
		        COALESCE(SUM(estimated_cost), 0)
		 FROM usage_logs
		 WHERE user_id = ? AND created_at >= datetime('now', ? || ' hours')
		 GROUP BY hour ORDER BY hour`,
		userID, -hours,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []HourlyUsage
	for rows.Next() {
		var h HourlyUsage
		if err := rows.Scan(&h.Hour, &h.PromptTokens, &h.CompletionTokens, &h.EstimatedCost); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

// GetModelBreakdown returns usage aggregated by model for the last N days.
func (db *DB) GetModelBreakdown(userID int, days int) ([]ModelBreakdown, error) {
	rows, err := db.conn.Query(
		`SELECT COALESCE(model, 'unknown'), COUNT(*),
		        COALESCE(SUM(prompt_tokens), 0),
		        COALESCE(SUM(completion_tokens), 0),
		        COALESCE(SUM(total_tokens), 0),
		        COALESCE(SUM(estimated_cost), 0)
		 FROM usage_logs
		 WHERE user_id = ? AND created_at >= datetime('now', ? || ' days')
		 GROUP BY model ORDER BY SUM(estimated_cost) DESC`,
		userID, -days,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ModelBreakdown
	for rows.Next() {
		var m ModelBreakdown
		if err := rows.Scan(&m.Model, &m.RequestCount, &m.PromptTokens, &m.CompletionTokens, &m.TotalTokens, &m.EstimatedCost); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
