package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	_ "modernc.org/sqlite"
)

var memdbSeq int64

// Open opens the SQLite database at path, creating parent directories as needed.
// It enables foreign keys and applies all schema migrations.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}

	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("applying migrations: %w", err)
	}

	return db, nil
}

// OpenMemory opens an in-memory SQLite database for testing.
// Each call creates an isolated database by using a unique name.
func OpenMemory() (*sql.DB, error) {
	id := atomic.AddInt64(&memdbSeq, 1)
	dsn := fmt.Sprintf("file:mneme_test_%d?mode=memory&cache=shared&_foreign_keys=on", id)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS notes (
  id           INTEGER PRIMARY KEY,
  path         TEXT UNIQUE NOT NULL,
  title        TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  indexed_at   TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS note_tags (
  note_id INTEGER NOT NULL REFERENCES notes(id),
  tag     TEXT NOT NULL,
  PRIMARY KEY (note_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_note_tags_tag ON note_tags(tag);

CREATE TABLE IF NOT EXISTS cards (
  id            INTEGER PRIMARY KEY,
  note_id       INTEGER NOT NULL REFERENCES notes(id),
  sentence_hash TEXT NOT NULL,
  cloze_key     TEXT NOT NULL DEFAULT '',
  kind          TEXT NOT NULL,
  front         TEXT NOT NULL,
  back          TEXT NOT NULL,
  status        TEXT NOT NULL,
  source        TEXT NOT NULL,
  created_at    TIMESTAMP NOT NULL,
  UNIQUE(note_id, sentence_hash, cloze_key)
);

CREATE TABLE IF NOT EXISTS reviews (
  card_id     INTEGER PRIMARY KEY REFERENCES cards(id),
  stability   REAL,
  difficulty  REAL,
  due         TIMESTAMP,
  state       TEXT,
  reps        INTEGER NOT NULL DEFAULT 0,
  lapses      INTEGER NOT NULL DEFAULT 0,
  last_review TIMESTAMP
);

CREATE TABLE IF NOT EXISTS review_log (
  id          INTEGER PRIMARY KEY,
  card_id     INTEGER NOT NULL REFERENCES cards(id),
  grade       INTEGER NOT NULL,
  reviewed_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS resolution_log (
  id             INTEGER PRIMARY KEY,
  batch_id       TEXT NOT NULL,
  action         TEXT NOT NULL,
  orphan_card_id INTEGER NOT NULL,
  target_card_id INTEGER,
  created_at     TIMESTAMP NOT NULL
);
`

func applyMigrations(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(schema); err != nil {
		return fmt.Errorf("executing schema: %w", err)
	}

	return tx.Commit()
}
