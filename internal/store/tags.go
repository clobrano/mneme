package store

import (
	"database/sql"
	"fmt"
)

// NoteTagRepository handles tag operations for notes.
type NoteTagRepository struct {
	db *sql.DB
}

// NewNoteTagRepository creates a new NoteTagRepository.
func NewNoteTagRepository(db *sql.DB) *NoteTagRepository {
	return &NoteTagRepository{db: db}
}

// Replace deletes all existing tags for a note and inserts the new list.
// The operation is wrapped in a transaction.
func (r *NoteTagRepository) Replace(noteID int64, tags []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM note_tags WHERE note_id = ?`, noteID); err != nil {
		return fmt.Errorf("delete tags for note %d: %w", noteID, err)
	}

	for _, tag := range tags {
		if _, err := tx.Exec(`INSERT INTO note_tags (note_id, tag) VALUES (?, ?)`, noteID, tag); err != nil {
			return fmt.Errorf("insert tag %q for note %d: %w", tag, noteID, err)
		}
	}

	return tx.Commit()
}

// GetTagsByNoteID returns all tags for a note.
func (r *NoteTagRepository) GetTagsByNoteID(noteID int64) ([]string, error) {
	rows, err := r.db.Query(`SELECT tag FROM note_tags WHERE note_id = ?`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}
