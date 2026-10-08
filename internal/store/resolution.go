package store

import (
	"database/sql"
	"fmt"
	"time"
)

// ResolutionEntry represents a single entry in the resolution log.
type ResolutionEntry struct {
	ID           int64
	BatchID      string
	Action       string
	OrphanCardID int64
	TargetCardID *int64
	CreatedAt    time.Time
}

// ResolutionLogRepository handles resolution log operations.
type ResolutionLogRepository struct {
	db *sql.DB
}

// NewResolutionLogRepository creates a new ResolutionLogRepository.
func NewResolutionLogRepository(db *sql.DB) *ResolutionLogRepository {
	return &ResolutionLogRepository{db: db}
}

// Insert adds a resolution log entry.
func (r *ResolutionLogRepository) Insert(batchID, action string, orphanCardID int64, targetCardID *int64) error {
	_, err := r.db.Exec(`
		INSERT INTO resolution_log (batch_id, action, orphan_card_id, target_card_id, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, batchID, action, orphanCardID, targetCardID, time.Now().UTC().Format(time.RFC3339))
	return err
}

// GetLastBatch returns all entries of the most recent batch, ordered by ID.
func (r *ResolutionLogRepository) GetLastBatch() ([]ResolutionEntry, error) {
	// First find the most recent batch_id
	var batchID string
	err := r.db.QueryRow(`
		SELECT batch_id FROM resolution_log ORDER BY id DESC LIMIT 1
	`).Scan(&batchID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get last batch id: %w", err)
	}

	return r.GetBatch(batchID)
}

// GetBatch returns all entries for a specific batch, ordered by ID.
func (r *ResolutionLogRepository) GetBatch(batchID string) ([]ResolutionEntry, error) {
	rows, err := r.db.Query(`
		SELECT id, batch_id, action, orphan_card_id, target_card_id, created_at
		FROM resolution_log WHERE batch_id = ? ORDER BY id ASC
	`, batchID)
	if err != nil {
		return nil, fmt.Errorf("get batch %s: %w", batchID, err)
	}
	defer rows.Close()

	var entries []ResolutionEntry
	for rows.Next() {
		var e ResolutionEntry
		var targetID sql.NullInt64
		var createdAt string
		if err := rows.Scan(&e.ID, &e.BatchID, &e.Action, &e.OrphanCardID, &targetID, &createdAt); err != nil {
			return nil, err
		}
		if targetID.Valid {
			id := targetID.Int64
			e.TargetCardID = &id
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// DeleteBatch removes all entries for a batch (used during undo).
func (r *ResolutionLogRepository) DeleteBatch(batchID string) error {
	_, err := r.db.Exec(`DELETE FROM resolution_log WHERE batch_id = ?`, batchID)
	return err
}

// Remap performs the full remap transaction: moves review history from orphan to target.
// This must never be called from sync — only from the resolver.
func Remap(db *sql.DB, orphanID, targetID int64, batchID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Step 1: Delete target's blank reviews row if reps==0
	_, err = tx.Exec(`DELETE FROM reviews WHERE card_id = ? AND reps = 0`, targetID)
	if err != nil {
		return fmt.Errorf("delete blank target review: %w", err)
	}

	// Step 2: Repoint reviews from orphan to target
	_, err = tx.Exec(`UPDATE reviews SET card_id = ? WHERE card_id = ?`, targetID, orphanID)
	if err != nil {
		return fmt.Errorf("repoint reviews: %w", err)
	}

	// Step 3: Repoint review_log from orphan to target
	_, err = tx.Exec(`UPDATE review_log SET card_id = ? WHERE card_id = ?`, targetID, orphanID)
	if err != nil {
		return fmt.Errorf("repoint review_log: %w", err)
	}

	// Step 4: Set orphan status to superseded
	_, err = tx.Exec(`UPDATE cards SET status = 'superseded' WHERE id = ?`, orphanID)
	if err != nil {
		return fmt.Errorf("supersede orphan: %w", err)
	}

	// Step 5: Insert resolution_log entry
	_, err = tx.Exec(`
		INSERT INTO resolution_log (batch_id, action, orphan_card_id, target_card_id, created_at)
		VALUES (?, 'remap', ?, ?, ?)
	`, batchID, orphanID, targetID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert resolution_log: %w", err)
	}

	return tx.Commit()
}

// Discard marks an orphan card as superseded and logs the action.
// This must never be called from sync — only from the resolver.
func Discard(db *sql.DB, orphanID int64, batchID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE cards SET status = 'superseded' WHERE id = ?`, orphanID)
	if err != nil {
		return fmt.Errorf("supersede orphan: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO resolution_log (batch_id, action, orphan_card_id, target_card_id, created_at)
		VALUES (?, 'discard', ?, NULL, ?)
	`, batchID, orphanID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert resolution_log: %w", err)
	}

	return tx.Commit()
}

// UndoLastBatch reverses the most recent resolution batch.
func UndoLastBatch(db *sql.DB) error {
	repo := NewResolutionLogRepository(db)
	entries, err := repo.GetLastBatch()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("no resolution batch to undo")
	}

	batchID := entries[0].BatchID

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Iterate in reverse order
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		switch e.Action {
		case "remap":
			if e.TargetCardID == nil {
				continue
			}
			// Repoint reviews back to orphan
			if _, err := tx.Exec(`UPDATE reviews SET card_id = ? WHERE card_id = ?`, e.OrphanCardID, *e.TargetCardID); err != nil {
				return fmt.Errorf("undo remap reviews: %w", err)
			}
			// Repoint review_log back to orphan
			if _, err := tx.Exec(`UPDATE review_log SET card_id = ? WHERE card_id = ?`, e.OrphanCardID, *e.TargetCardID); err != nil {
				return fmt.Errorf("undo remap review_log: %w", err)
			}
			// Set orphan back to orphaned
			if _, err := tx.Exec(`UPDATE cards SET status = 'orphaned' WHERE id = ?`, e.OrphanCardID); err != nil {
				return fmt.Errorf("undo orphan status: %w", err)
			}
			// Set target back to active
			if _, err := tx.Exec(`UPDATE cards SET status = 'active' WHERE id = ?`, *e.TargetCardID); err != nil {
				return fmt.Errorf("undo target status: %w", err)
			}
			// Re-create a blank reviews row for target
			now := time.Now().UTC()
			if _, err := tx.Exec(`
				INSERT OR IGNORE INTO reviews (card_id, stability, difficulty, due, state, reps, lapses)
				VALUES (?, 0, 0, ?, 'new', 0, 0)
			`, *e.TargetCardID, now.Format(time.RFC3339)); err != nil {
				return fmt.Errorf("restore target review: %w", err)
			}

		case "discard":
			// Set orphan back to orphaned
			if _, err := tx.Exec(`UPDATE cards SET status = 'orphaned' WHERE id = ?`, e.OrphanCardID); err != nil {
				return fmt.Errorf("undo discard: %w", err)
			}
		}
	}

	// Delete the batch
	if _, err := tx.Exec(`DELETE FROM resolution_log WHERE batch_id = ?`, batchID); err != nil {
		return fmt.Errorf("delete batch: %w", err)
	}

	return tx.Commit()
}
