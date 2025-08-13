package storage

import (
	"database/sql"
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
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS messages(
		chat_id INTEGER, user_id INTEGER, text TEXT, ts INTEGER
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
