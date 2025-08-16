package storage

import (
	"database/sql"
	"time"

	// Register sqlite3 driver
	_ "github.com/mattn/go-sqlite3"
)

type DB interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	Close() error
}

type Store struct{ db DB }

func OpenSQLite(dsn string) (DB, error) {
	return sql.Open("sqlite3", dsn)
}

func InitSchema(db DB) error {
	// Create messages table
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS messages(
		chat_id INTEGER, user_id INTEGER, text TEXT, ts INTEGER
	)`); err != nil {
		return err
	}

	// Create command_usage table for analytics
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS command_usage(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER,
		user_id INTEGER,
		command TEXT,
		category TEXT,
		ts INTEGER
	)`)
	return err
}

func NewStore(db DB) *Store { return &Store{db: db} }

func (s *Store) SaveMessage(chatID, userID int64, text string, ts int64) error {
	_, err := s.db.Exec(`INSERT INTO messages(chat_id,user_id,text,ts) VALUES(?,?,?,?)`,
		chatID, userID, text, ts)
	return err
}

func (s *Store) FetchMessages(chatID int64, since int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT text FROM messages WHERE chat_id=? AND ts>=? ORDER BY ts ASC`,
		chatID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil && t != "" {
			out = append(out, t)
		}
	}
	return out, nil
}

// CommandUsage represents a command usage record
type CommandUsage struct {
	Command   string
	Category  string
	ChatID    int64
	UserID    int64
	Timestamp int64
}

// SaveCommandUsage tracks command usage for analytics
func (s *Store) SaveCommandUsage(chatID, userID int64, command, category string) error {
	ts := time.Now().Unix()
	_, err := s.db.Exec(`INSERT INTO command_usage(chat_id,user_id,command,category,ts) VALUES(?,?,?,?,?)`,
		chatID, userID, command, category, ts)
	return err
}

// UsageStats represents aggregated usage statistics
type UsageStats struct {
	Category string
	Count    int
	Commands map[string]int // command -> count
}

// FetchUsageStats retrieves usage statistics for the given time period
func (s *Store) FetchUsageStats(chatID int64, since int64) (map[string]*UsageStats, error) {
	rows, err := s.db.Query(`
		SELECT category, command, COUNT(*) as count 
		FROM command_usage 
		WHERE chat_id=? AND ts>=? 
		GROUP BY category, command 
		ORDER BY category, count DESC`,
		chatID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]*UsageStats)
	for rows.Next() {
		var category, command string
		var count int
		if err := rows.Scan(&category, &command, &count); err != nil {
			continue
		}

		if stats[category] == nil {
			stats[category] = &UsageStats{
				Category: category,
				Commands: make(map[string]int),
			}
		}
		stats[category].Commands[command] = count
		stats[category].Count += count
	}
	return stats, nil
}

// TimeSeriesPoint represents a point in time series data
type TimeSeriesPoint struct {
	Timestamp int64
	Count     int
}

// FetchUsageTimeSeries retrieves time series data for usage analytics
func (s *Store) FetchUsageTimeSeries(chatID int64, since int64, intervalHours int) (map[string][]TimeSeriesPoint, error) {
	// Group by time intervals (default 1 hour)
	if intervalHours <= 0 {
		intervalHours = 1
	}

	rows, err := s.db.Query(`
		SELECT 
			category,
			(ts / (? * 3600)) * (? * 3600) as time_bucket,
			COUNT(*) as count
		FROM command_usage 
		WHERE chat_id=? AND ts>=? 
		GROUP BY category, time_bucket 
		ORDER BY category, time_bucket`,
		intervalHours, intervalHours, chatID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	series := make(map[string][]TimeSeriesPoint)
	for rows.Next() {
		var category string
		var timestamp int64
		var count int
		if err := rows.Scan(&category, &timestamp, &count); err != nil {
			continue
		}

		series[category] = append(series[category], TimeSeriesPoint{
			Timestamp: timestamp,
			Count:     count,
		})
	}
	return series, nil
}
