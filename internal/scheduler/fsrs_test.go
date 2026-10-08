package scheduler

import (
	"testing"
	"time"

	"github.com/clobrano/mneme/internal/store"
)

func newReview(cardID int64) *store.DBReview {
	now := time.Now()
	return &store.DBReview{
		CardID:     cardID,
		Stability:  0,
		Difficulty: 0,
		Due:        now,
		State:      "new",
		Reps:       0,
		Lapses:     0,
	}
}

// TestAgainIncrementsLapses: grading again (1) on a review-state card increments lapses.
func TestAgainIncrementsLapses(t *testing.T) {
	now := time.Now()
	rev := &store.DBReview{
		CardID:     1,
		Stability:  4.0,
		Difficulty: 5.0,
		Due:        now,
		State:      "review",
		Reps:       3,
		Lapses:     0,
	}

	updated := Grade(rev, 1, now)

	if updated.Lapses <= rev.Lapses {
		t.Errorf("lapses should increase after again: was %d, got %d", rev.Lapses, updated.Lapses)
	}
	if updated.Due.After(now.Add(24 * time.Hour)) {
		t.Errorf("due after again should be soon, got %v", updated.Due)
	}
}

// TestGoodAdvancesDue: grading good (3) on a new card advances due date.
func TestGoodAdvancesDue(t *testing.T) {
	now := time.Now()
	rev := newReview(2)

	updated := Grade(rev, 3, now)

	if !updated.Due.After(now) {
		t.Errorf("due should be in the future after good, got %v", updated.Due)
	}
	if updated.State == "new" {
		t.Errorf("state should advance past new after good, got %q", updated.State)
	}
}

// TestAgainShortensInterval compared to good.
func TestAgainShortensIntervalVsGood(t *testing.T) {
	now := time.Now()

	rev1 := &store.DBReview{
		CardID:     3,
		Stability:  10.0,
		Difficulty: 5.0,
		Due:        now,
		State:      "review",
		Reps:       5,
		Lapses:     0,
	}
	rev2 := &store.DBReview{
		CardID:     4,
		Stability:  10.0,
		Difficulty: 5.0,
		Due:        now,
		State:      "review",
		Reps:       5,
		Lapses:     0,
	}

	again := Grade(rev1, 1, now)
	good := Grade(rev2, 3, now)

	if !again.Due.Before(good.Due) {
		t.Errorf("again.Due (%v) should be before good.Due (%v)", again.Due, good.Due)
	}
}

// TestEasyAdvancesMore: easy advances due more than good.
func TestEasyAdvancesMore(t *testing.T) {
	now := time.Now()

	rev1 := &store.DBReview{
		CardID:     5,
		Stability:  10.0,
		Difficulty: 5.0,
		Due:        now,
		State:      "review",
		Reps:       5,
		Lapses:     0,
	}
	rev2 := &store.DBReview{
		CardID:     6,
		Stability:  10.0,
		Difficulty: 5.0,
		Due:        now,
		State:      "review",
		Reps:       5,
		Lapses:     0,
	}

	easy := Grade(rev1, 4, now)
	good := Grade(rev2, 3, now)

	if !easy.Due.After(good.Due) {
		t.Errorf("easy.Due (%v) should be after good.Due (%v)", easy.Due, good.Due)
	}
}
