package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DBCard is the database representation of a card.
type DBCard struct {
	ID           int64
	NoteID       int64
	SentenceHash string
	ClozeKey     string
	Kind         string
	Front        string
	Back         string
	Status       string
	Source       string
	CreatedAt    time.Time
}

// CardRepository handles database operations for cards.
type CardRepository struct {
	db *sql.DB
}

// NewCardRepository creates a new CardRepository.
func NewCardRepository(db *sql.DB) *CardRepository {
	return &CardRepository{db: db}
}

// Upsert inserts a card if it doesn't exist (by the UNIQUE constraint).
// Uses INSERT OR IGNORE so RowsAffected()==1 unambiguously means a new row.
// Returns the card ID and whether it was newly inserted.
func (r *CardRepository) Upsert(noteID int64, sentenceHash, clozeKey, kind, front, back, source string) (int64, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`
		INSERT OR IGNORE INTO cards (note_id, sentence_hash, cloze_key, kind, front, back, status, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?)
	`, noteID, sentenceHash, clozeKey, kind, front, back, source, now)
	if err != nil {
		return 0, false, fmt.Errorf("upsert card: %w", err)
	}

	rowsAff, _ := res.RowsAffected()
	if rowsAff == 1 {
		id, _ := res.LastInsertId()
		return id, true, nil
	}

	// Row already exists — get its ID and reactivate if it was orphaned.
	var id int64
	err = r.db.QueryRow(
		`SELECT id FROM cards WHERE note_id=? AND sentence_hash=? AND cloze_key=?`,
		noteID, sentenceHash, clozeKey,
	).Scan(&id)
	if err != nil {
		return 0, false, err
	}
	r.db.Exec(`UPDATE cards SET kind=?, front=?, back=?, status='active' WHERE id=? AND status='orphaned'`,
		kind, front, back, id)
	return id, false, nil
}

// GetByNoteID returns all cards for a note.
func (r *CardRepository) GetByNoteID(noteID int64) ([]*DBCard, error) {
	return r.queryCards(`SELECT id, note_id, sentence_hash, cloze_key, kind, front, back, status, source, created_at FROM cards WHERE note_id = ?`, noteID)
}

// ListActiveByNoteID returns active cards for a note.
func (r *CardRepository) ListActiveByNoteID(noteID int64) ([]*DBCard, error) {
	return r.queryCards(`SELECT id, note_id, sentence_hash, cloze_key, kind, front, back, status, source, created_at FROM cards WHERE note_id = ? AND status = 'active'`, noteID)
}

// SetStatus updates the status of a card.
func (r *CardRepository) SetStatus(cardID int64, status string) error {
	_, err := r.db.Exec(`UPDATE cards SET status = ? WHERE id = ?`, status, cardID)
	return err
}

// ListOrphaned returns all orphaned cards.
func (r *CardRepository) ListOrphaned() ([]*DBCard, error) {
	return r.queryCards(`SELECT id, note_id, sentence_hash, cloze_key, kind, front, back, status, source, created_at FROM cards WHERE status = 'orphaned'`)
}

// ListActive returns all active cards.
func (r *CardRepository) ListActive() ([]*DBCard, error) {
	return r.queryCards(`SELECT id, note_id, sentence_hash, cloze_key, kind, front, back, status, source, created_at FROM cards WHERE status = 'active'`)
}

// DueCards returns cards due for review, with optional tag filtering.
func DueCards(db *sql.DB, now time.Time, tags []string, matchMode string, excludeTags []string, noteTitle string, limit int) ([]*DBCard, error) {
	nowStr := now.UTC().Format(time.RFC3339)

	var conditions []string
	var args []interface{}

	conditions = append(conditions, "c.status = 'active'")
	conditions = append(conditions, "(r.due IS NULL OR r.due <= ?)")
	args = append(args, nowStr)

	if noteTitle != "" {
		conditions = append(conditions, "n.title LIKE ?")
		args = append(args, "%"+noteTitle+"%")
	}

	// Tag filtering
	if len(tags) > 0 {
		if matchMode == "all" {
			// Intersection: note must have all tags
			for _, tag := range tags {
				conditions = append(conditions, fmt.Sprintf(`EXISTS (
					SELECT 1 FROM note_tags nt WHERE nt.note_id = c.note_id AND nt.tag = ?
				)`))
				args = append(args, strings.ToLower(tag))
			}
		} else {
			// Union: note must have any tag
			placeholders := make([]string, len(tags))
			for i, tag := range tags {
				placeholders[i] = "?"
				args = append(args, strings.ToLower(tag))
			}
			conditions = append(conditions, fmt.Sprintf(`EXISTS (
				SELECT 1 FROM note_tags nt WHERE nt.note_id = c.note_id AND nt.tag IN (%s)
			)`, strings.Join(placeholders, ",")))
		}
	}

	// Exclude tags
	for _, tag := range excludeTags {
		conditions = append(conditions, fmt.Sprintf(`NOT EXISTS (
			SELECT 1 FROM note_tags nt WHERE nt.note_id = c.note_id AND nt.tag = ?
		)`))
		args = append(args, strings.ToLower(tag))
	}

	query := fmt.Sprintf(`
		SELECT c.id, c.note_id, c.sentence_hash, c.cloze_key, c.kind, c.front, c.back, c.status, c.source, c.created_at
		FROM cards c
		JOIN notes n ON n.id = c.note_id
		LEFT JOIN reviews r ON r.card_id = c.id
		WHERE %s
		ORDER BY r.due ASC NULLS FIRST
	`, strings.Join(conditions, " AND "))

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("due cards query: %w", err)
	}
	defer rows.Close()

	var cards []*DBCard
	for rows.Next() {
		c := &DBCard{}
		var createdAt string
		if err := rows.Scan(&c.ID, &c.NoteID, &c.SentenceHash, &c.ClozeKey, &c.Kind, &c.Front, &c.Back, &c.Status, &c.Source, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func (r *CardRepository) queryCards(query string, args ...interface{}) ([]*DBCard, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query cards: %w", err)
	}
	defer rows.Close()

	var cards []*DBCard
	for rows.Next() {
		c := &DBCard{}
		var createdAt string
		if err := rows.Scan(&c.ID, &c.NoteID, &c.SentenceHash, &c.ClozeKey, &c.Kind, &c.Front, &c.Back, &c.Status, &c.Source, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// CountActive returns the number of active cards.
func CountActive(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM cards WHERE status = 'active'`).Scan(&n)
	return n, err
}

// CountOrphaned returns the number of orphaned cards.
func CountOrphaned(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM cards WHERE status = 'orphaned'`).Scan(&n)
	return n, err
}

// CountDueToday returns the number of cards due today.
func CountDueToday(db *sql.DB, endOfDay time.Time) (int, error) {
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM cards c
		JOIN reviews r ON r.card_id = c.id
		WHERE r.due <= ? AND c.status = 'active'
	`, endOfDay.UTC().Format(time.RFC3339)).Scan(&n)
	return n, err
}

// ForecastEntry holds the due count for a day.
type ForecastEntry struct {
	Date  string
	Count int
}

// DueForecast returns a 7-day forecast of due cards.
func DueForecast(db *sql.DB, startDate time.Time) ([]ForecastEntry, error) {
	rows, err := db.Query(`
		SELECT DATE(r.due) as day, COUNT(*) as cnt
		FROM cards c
		JOIN reviews r ON r.card_id = c.id
		WHERE c.status = 'active'
		  AND r.due >= ?
		  AND r.due < ?
		GROUP BY day
		ORDER BY day
	`,
		startDate.UTC().Format("2006-01-02"),
		startDate.AddDate(0, 0, 7).UTC().Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ForecastEntry
	for rows.Next() {
		var e ForecastEntry
		if err := rows.Scan(&e.Date, &e.Count); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
