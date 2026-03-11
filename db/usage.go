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
}

// UsageStats holds aggregated usage data for a time period.
type UsageStats struct {
	Period           string
	RequestCount     int
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}

// RecentRequest represents a single recent request for display.
type RecentRequest struct {
	Time             time.Time
	Path             string
	Model            string
	TotalTokens      int
}

// InsertUsage inserts a usage log record.
func (db *DB) InsertUsage(r UsageRecord) error {
	_, err := db.conn.Exec(
		`INSERT INTO usage_logs (user_id, model, path, prompt_tokens, completion_tokens, total_tokens)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.UserID, r.Model, r.Path, r.PromptTokens, r.CompletionTokens, r.TotalTokens,
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
			`SELECT COUNT(*), COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0), COALESCE(SUM(total_tokens), 0)
			 FROM usage_logs WHERE user_id = ? AND created_at >= datetime('now', ?)`,
			userID, p.sqlArg,
		).Scan(&s.RequestCount, &s.PromptTokens, &s.CompletionTokens, &s.TotalTokens)
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
		`SELECT created_at, path, model, total_tokens FROM usage_logs
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
		if err := rows.Scan(&r.Time, &r.Path, &r.Model, &r.TotalTokens); err != nil {
			return nil, err
		}
		reqs = append(reqs, r)
	}
	return reqs, rows.Err()
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
