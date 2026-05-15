package dbstore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open() (*Store, error) {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "./data"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "app.sqlite")
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS seen_events (
  event_key TEXT PRIMARY KEY,
  first_seen_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS subscribers (
  email TEXT PRIMARY KEY,
  created_at TEXT NOT NULL
);`); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) IsNewEvent(key string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM seen_events WHERE event_key = ?)`, key).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists == 0, nil
}

func (s *Store) MarkEventSeen(key, atISO string) error {
	if atISO == "" {
		atISO = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO seen_events (event_key, first_seen_at) VALUES (?, ?)`, key, atISO)
	return err
}

func (s *Store) SeenCount() (int64, error) {
	var c int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM seen_events`).Scan(&c)
	return c, err
}

func (s *Store) FirstSeenAt(key string) (string, error) {
	var t string
	err := s.db.QueryRow(`SELECT first_seen_at FROM seen_events WHERE event_key = ?`, key).Scan(&t)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return t, err
}

type Subscriber struct {
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) ListSubscribers() ([]Subscriber, error) {
	rows, err := s.db.Query(`SELECT email, created_at FROM subscribers ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Subscriber
	for rows.Next() {
		var r Subscriber
		if err := rows.Scan(&r.Email, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) AddSubscriber(email string) (string, error) {
	e := strings.TrimSpace(strings.ToLower(email))
	if e == "" || !strings.Contains(e, "@") {
		return "", fmt.Errorf("email invalide")
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO subscribers (email, created_at) VALUES (?, ?)`,
		e, time.Now().UTC().Format(time.RFC3339Nano))
	return e, err
}

func (s *Store) RemoveSubscriber(email string) error {
	e := strings.TrimSpace(strings.ToLower(email))
	_, err := s.db.Exec(`DELETE FROM subscribers WHERE email = ?`, e)
	return err
}

func (s *Store) SubscriberCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM subscribers`).Scan(&n)
	return n, err
}
