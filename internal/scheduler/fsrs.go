package scheduler

import (
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v3"

	"github.com/clobrano/mneme/internal/store"
)

// Grade applies FSRS scheduling to a review given a grade (1=again, 2=hard, 3=good, 4=easy).
// It returns the updated review row with new scheduling fields.
func Grade(current *store.DBReview, grade int, now time.Time) *store.DBReview {
	f := fsrs.DefaultParam()
	algo := fsrs.NewFSRS(f)

	card := fsrs.Card{
		Due:        current.Due,
		Stability:  current.Stability,
		Difficulty: current.Difficulty,
		Reps:       uint64(current.Reps),
		Lapses:     uint64(current.Lapses),
		LastReview: now,
	}

	switch current.State {
	case "new":
		card.State = fsrs.New
	case "learning":
		card.State = fsrs.Learning
	case "review":
		card.State = fsrs.Review
	case "relearning":
		card.State = fsrs.Relearning
	default:
		card.State = fsrs.New
	}

	schedulingCards := algo.Repeat(card, now)

	var rating fsrs.Rating
	switch grade {
	case 1:
		rating = fsrs.Again
	case 2:
		rating = fsrs.Hard
	case 3:
		rating = fsrs.Good
	case 4:
		rating = fsrs.Easy
	default:
		rating = fsrs.Good
	}

	result := schedulingCards[rating].Card

	updated := &store.DBReview{
		CardID:     current.CardID,
		Stability:  result.Stability,
		Difficulty: result.Difficulty,
		Due:        result.Due,
		Reps:       int(result.Reps),
		Lapses:     int(result.Lapses),
	}
	updated.LastReview = &now

	switch result.State {
	case fsrs.New:
		updated.State = "new"
	case fsrs.Learning:
		updated.State = "learning"
	case fsrs.Review:
		updated.State = "review"
	case fsrs.Relearning:
		updated.State = "relearning"
	}

	return updated
}
