package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/clobrano/mneme/internal/store"
)

// RunOrphans implements the `orphans` subcommand.
func RunOrphans(opts Options, args []string) int {
	db, err := store.Open(opts.Config.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening db: %v\n", err)
		return 1
	}
	defer db.Close()

	cardRepo := store.NewCardRepository(db)
	reviewRepo := store.NewReviewRepository(db)

	orphans, err := cardRepo.ListOrphaned()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing orphans: %v\n", err)
		return 1
	}

	if opts.JSONOutput {
		type orphanEntry struct {
			ID           int64  `json:"id"`
			Sentence     string `json:"sentence"`
			Kind         string `json:"kind"`
			Reps         int    `json:"reps"`
			Lapses       int    `json:"lapses"`
			Due          string `json:"due"`
		}
		var entries []orphanEntry
		for _, c := range orphans {
			rev, _ := reviewRepo.GetByCardID(c.ID)
			e := orphanEntry{ID: c.ID, Sentence: c.Back, Kind: c.Kind}
			if rev != nil {
				e.Reps = rev.Reps
				e.Lapses = rev.Lapses
				e.Due = rev.Due.Format("2006-01-02")
			}
			entries = append(entries, e)
		}
		b, _ := json.Marshal(entries)
		fmt.Println(string(b))
		return 0
	}

	if len(orphans) == 0 {
		fmt.Println("No orphaned cards.")
		return 0
	}

	fmt.Printf("%-6s %-8s %-6s %-6s %-12s %s\n", "ID", "KIND", "REPS", "LAPSES", "DUE", "SENTENCE")
	fmt.Printf("%-6s %-8s %-6s %-6s %-12s %s\n", "--", "----", "----", "------", "---", "--------")
	for _, c := range orphans {
		rev, _ := reviewRepo.GetByCardID(c.ID)
		reps, lapses, due := 0, 0, "—"
		if rev != nil {
			reps = rev.Reps
			lapses = rev.Lapses
			due = rev.Due.Format("2006-01-02")
		}
		sentence := c.Back
		if len(sentence) > 60 {
			sentence = sentence[:57] + "..."
		}
		fmt.Printf("%-6d %-8s %-6d %-6d %-12s %s\n", c.ID, c.Kind, reps, lapses, due, sentence)
	}
	return 0
}
