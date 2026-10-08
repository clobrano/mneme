package store

import (
	"database/sql"
	"fmt"
	"time"
)

// DBNote is the database representation of a note.
type DBNote struct {
	ID          int64
	Path        string
	Title       string
	ContentHash string
	IndexedAt   time.Time
}

// NoteRepository handles database operations for notes.
type NoteRepository struct {
	db *sql.DB
}

// NewNoteRepository creates a new NoteRepository.
func NewNoteRepository(db *sql.DB) *NoteRepository {
	return &NoteRepository{db: db}
}

// Upsert inserts or updates a note by path. Returns the note ID.
func (r *NoteRepository) Upsert(path, title, contentHash string) (int64, error) {
	now := time.Now().UTC()
	res, err := r.db.Exec(`
		INSERT INTO notes (path, title, content_hash, indexed_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title=excluded.title,
			content_hash=excluded.content_hash,
			indexed_at=excluded.indexed_at
	`, path, title, contentHash, now.Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("upsert note %s: %w", path, err)
	}

	// Try to get the last insert ID; for conflicts, use GetByPath.
	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		note, err := r.GetByPath(path)
		if err != nil {
			return 0, err
		}
		return note.ID, nil
	}
	return id, nil
}

// GetByPath retrieves a note by its file path.
func (r *NoteRepository) GetByPath(path string) (*DBNote, error) {
	note := &DBNote{}
	var indexedAt string
	err := r.db.QueryRow(
		`SELECT id, path, title, content_hash, indexed_at FROM notes WHERE path = ?`,
		path,
	).Scan(&note.ID, &note.Path, &note.Title, &note.ContentHash, &indexedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get note by path %s: %w", path, err)
	}
	note.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
	return note, nil
}

// ListAll returns all notes in the database.
func (r *NoteRepository) ListAll() ([]*DBNote, error) {
	rows, err := r.db.Query(`SELECT id, path, title, content_hash, indexed_at FROM notes`)
	if err != nil {
		return nil, fmt.Errorf("list all notes: %w", err)
	}
	defer rows.Close()

	var notes []*DBNote
	for rows.Next() {
		note := &DBNote{}
		var indexedAt string
		if err := rows.Scan(&note.ID, &note.Path, &note.Title, &note.ContentHash, &indexedAt); err != nil {
			return nil, err
		}
		note.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
		notes = append(notes, note)
	}
	return notes, rows.Err()
}
