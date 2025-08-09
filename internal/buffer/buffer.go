package buffer

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Message struct {
	ID        int64
	Payload   []byte
	CreatedAt time.Time
	Sent      bool
}

type Store struct {
	db *sql.DB
}

// Init opens/creates the sqlite DB and ensures schema exists.
func Init(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("buffer path is empty")
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, err
	}

	create := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		payload BLOB NOT NULL,
		created_at INTEGER NOT NULL,
		sent INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_messages_sent ON messages(sent);
	`
	if _, err := db.Exec(create); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Enqueue stores a payload on disk for later forwarding.
func (s *Store) Enqueue(payload []byte) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("store not initialized")
	}
	stmt, err := s.db.Prepare("INSERT INTO messages(payload, created_at, sent) VALUES (?, ?, 0)")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	res, err := stmt.Exec(payload, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FetchUnsent returns up to limit unsent messages ordered by id.
func (s *Store) FetchUnsent(limit int) ([]Message, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store not initialized")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query("SELECT id, payload, created_at, sent FROM messages WHERE sent=0 ORDER BY id LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var ts int64
		var payload []byte
		var sentInt int
		if err := rows.Scan(&m.ID, &payload, &ts, &sentInt); err != nil {
			return nil, err
		}
		m.Payload = payload
		m.CreatedAt = time.Unix(ts, 0)
		m.Sent = sentInt != 0
		out = append(out, m)
	}
	return out, nil
}

// CountUnsent returns the total number of unsent messages in the buffer.
func (s *Store) CountUnsent() (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("store not initialized")
	}
	var count int
	row := s.db.QueryRow("SELECT COUNT(1) FROM messages WHERE sent=0")
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// MarkSent marks a list of message ids as sent (sent=1).
func (s *Store) MarkSent(ids []int64) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("UPDATE messages SET sent=1 WHERE id = ?")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
