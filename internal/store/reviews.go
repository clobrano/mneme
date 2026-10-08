package store

import (
	"database/sql"
	"fmt"
	"time"
)

// DBReview is the database representation of a review state.
type DBReview struct {
	CardID     int64
	Stability  float64
	Difficulty float64
	Due        time.Time
	State      string
	Reps       int
	Lapses     int
	LastReview *time.Time
}

// ReviewRepository handles database operations for reviews.
type ReviewRepository struct {
	db *sql.DB
}

// NewReviewRepository creates a new ReviewRepository.
func NewReviewRepository(db *sql.DB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

// Create inserts a fresh review row for a new card (state=new, reps=0).
func (r *ReviewRepository) Create(cardID int64) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(`
		INSERT INTO reviews (card_id, stability, difficulty, due, state, reps, lapses, last_review)
		VALUES (?, 0, 0, ?, 'new', 0, 0, NULL)
	`, cardID, now.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("create review for card %d: %w", cardID, err)
	}
	return nil
}

// GetByCardID retrieves the review state for a card.
func (r *ReviewRepository) GetByCardID(cardID int64) (*DBReview, error) {
	rev := &DBReview{CardID: cardID}
	var due, lastReview sql.NullString
	err := r.db.QueryRow(`
		SELECT stability, difficulty, due, state, reps, lapses, last_review
		FROM reviews WHERE card_id = ?
	`, cardID).Scan(
		&rev.Stability, &rev.Difficulty, &due, &rev.State,
		&rev.Reps, &rev.Lapses, &lastReview,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get review for card %d: %w", cardID, err)
	}

	if due.Valid {
		t, _ := time.Parse(time.RFC3339, due.String)
		rev.Due = t
	}
	if lastReview.Valid {
		t, _ := time.Parse(time.RFC3339, lastReview.String)
		rev.LastReview = &t
	}

	return rev, nil
}

// Update replaces all scheduling fields for a review.
func (r *ReviewRepository) Update(rev *DBReview) error {
	var lastReview interface{}
	if rev.LastReview != nil {
		lastReview = rev.LastReview.UTC().Format(time.RFC3339)
	}
	_, err := r.db.Exec(`
		UPDATE reviews SET
			stability=?, difficulty=?, due=?, state=?, reps=?, lapses=?, last_review=?
		WHERE card_id=?
	`,
		rev.Stability, rev.Difficulty,
		rev.Due.UTC().Format(time.RFC3339),
		rev.State, rev.Reps, rev.Lapses,
		lastReview,
		rev.CardID,
	)
	return err
}

// ReviewLogRepository handles review log operations.
type ReviewLogRepository struct {
	db *sql.DB
}

// NewReviewLogRepository creates a new ReviewLogRepository.
func NewReviewLogRepository(db *sql.DB) *ReviewLogRepository {
	return &ReviewLogRepository{db: db}
}

// Insert adds a review log entry.
func (r *ReviewLogRepository) Insert(cardID int64, grade int) error {
	_, err := r.db.Exec(`
		INSERT INTO review_log (card_id, grade, reviewed_at) VALUES (?, ?, ?)
	`, cardID, grade, time.Now().UTC().Format(time.RFC3339))
	return err
}
