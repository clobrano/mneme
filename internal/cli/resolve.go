package cli

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/clobrano/mneme/internal/store"
	"github.com/clobrano/mneme/internal/tui"
)

// RunResolve implements the `resolve` subcommand.
func RunResolve(opts Options, args []string) int {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	undo := fs.Bool("undo", false, "undo the last resolution batch")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	db, err := store.Open(opts.Config.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening db: %v\n", err)
		return 1
	}
	defer db.Close()

	if *undo {
		return runResolveUndo(db)
	}

	// Use TUI when terminal is detected and --no-tui not set
	if !opts.NoTUI && term.IsTerminal(int(os.Stdout.Fd())) {
		return runResolveTUI(db)
	}
	return resolvePlain(db)
}

func runResolveTUI(db *sql.DB) int {
	if err := tui.RunResolveTUI(db); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		return 1
	}
	return 0
}

func runResolveUndo(db *sql.DB) int {
	if err := store.UndoLastBatch(db); err != nil {
		fmt.Fprintf(os.Stderr, "undo error: %v\n", err)
		return 1
	}
	fmt.Println("Last resolution batch undone.")
	return 0
}

func resolvePlain(db *sql.DB) int {
	cardRepo := store.NewCardRepository(db)
	reviewRepo := store.NewReviewRepository(db)
	noteRepo := store.NewNoteRepository(db)

	orphans, err := cardRepo.ListOrphaned()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing orphans: %v\n", err)
		return 1
	}

	if len(orphans) == 0 {
		fmt.Println("No orphaned cards to resolve.")
		return 0
	}

	batchID := fmt.Sprintf("%d", time.Now().UnixNano())
	reader := bufio.NewReader(os.Stdin)

	for _, orphan := range orphans {
		rev, _ := reviewRepo.GetByCardID(orphan.ID)

		fmt.Println("\n" + strings.Repeat("─", 60))
		fmt.Printf("Orphan [%d] (%s)\n", orphan.ID, orphan.Kind)
		fmt.Printf("  Sentence: %s\n", orphan.Back)
		if rev != nil {
			fmt.Printf("  History:  reps=%d  lapses=%d  due=%s\n",
				rev.Reps, rev.Lapses, rev.Due.Format("2006-01-02"))
		}
		fmt.Println()

		// Get candidates: default to same note, with widening options
		candidates, err := cardRepo.ListActiveByNoteID(orphan.NoteID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error getting candidates: %v\n", err)
			continue
		}

		dbNote, _ := noteRepo.GetByPath("")
		_ = dbNote

		widened := false
		keyword := ""

		for {
			displayCandidates(candidates, noteRepo, widened, keyword)

			fmt.Println()
			fmt.Println("Actions: r<n>y=remap  dy=discard  s=skip  w=widen  /<keyword>=search")
			fmt.Print("> ")

			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)

			if line == "" {
				// Enter does nothing (safety requirement)
				continue
			}

			if line == "s" {
				fmt.Println("Skipped.")
				break
			}

			if line == "dy" {
				if err := store.Discard(db, orphan.ID, batchID); err != nil {
					fmt.Fprintf(os.Stderr, "discard error: %v\n", err)
				} else {
					fmt.Println("Discarded.")
				}
				break
			}

			if line == "d" {
				// Incomplete discard — need confirmation 'y'
				fmt.Print("Confirm discard (y to confirm): ")
				confirm, _ := reader.ReadString('\n')
				confirm = strings.TrimSpace(confirm)
				if confirm == "y" {
					if err := store.Discard(db, orphan.ID, batchID); err != nil {
						fmt.Fprintf(os.Stderr, "discard error: %v\n", err)
					} else {
						fmt.Println("Discarded.")
					}
					break
				}
				fmt.Println("Discard cancelled.")
				continue
			}

			if line == "w" {
				// Widen to recently-created cards across all notes
				widened = true
				candidates, err = getRecentCards(db)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error getting recent cards: %v\n", err)
				}
				continue
			}

			if strings.HasPrefix(line, "/") {
				keyword = strings.TrimPrefix(line, "/")
				candidates, err = searchCards(db, keyword)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error searching cards: %v\n", err)
				}
				continue
			}

			// Parse remap: r<n>y  (e.g. "r3y")
			if strings.HasPrefix(line, "r") {
				rest := strings.TrimPrefix(line, "r")
				if strings.HasSuffix(rest, "y") {
					numStr := strings.TrimSuffix(rest, "y")
					idx, err := strconv.Atoi(numStr)
					if err == nil && idx >= 1 && idx <= len(candidates) {
						target := candidates[idx-1]
						if err := store.Remap(db, orphan.ID, target.ID, batchID); err != nil {
							fmt.Fprintf(os.Stderr, "remap error: %v\n", err)
						} else {
							fmt.Printf("Remapped to card [%d].\n", target.ID)
						}
						break
					}
				}
				// r without y — print prompt again
				fmt.Println("Use r<number>y to remap (e.g. r1y). No pre-selection; y confirms.")
				continue
			}

			fmt.Println("Unknown action. Use r<n>y, dy, s, w, or /<keyword>.")
		}
	}

	fmt.Println("\nResolution complete.")
	return 0
}

func displayCandidates(candidates []*store.DBCard, noteRepo *store.NoteRepository, widened bool, keyword string) {
	if len(candidates) == 0 {
		fmt.Println("  (no candidates)")
		return
	}
	label := "Same-note candidates"
	if widened {
		label = "All recent candidates"
	} else if keyword != "" {
		label = fmt.Sprintf("Candidates matching %q", keyword)
	}
	fmt.Printf("%s:\n", label)
	for i, c := range candidates {
		sentence := c.Back
		if len(sentence) > 60 {
			sentence = sentence[:57] + "..."
		}
		fmt.Printf("  [%d] (%s) %s\n", i+1, c.Kind, sentence)
	}
}

func getRecentCards(db *sql.DB) ([]*store.DBCard, error) {
	rows, err := db.Query(`
		SELECT id, note_id, sentence_hash, cloze_key, kind, front, back, status, source, created_at
		FROM cards WHERE status = 'active'
		ORDER BY created_at DESC LIMIT 20
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCards(rows)
}

func searchCards(db *sql.DB, keyword string) ([]*store.DBCard, error) {
	rows, err := db.Query(`
		SELECT c.id, c.note_id, c.sentence_hash, c.cloze_key, c.kind, c.front, c.back, c.status, c.source, c.created_at
		FROM cards c
		JOIN notes n ON n.id = c.note_id
		WHERE c.status = 'active'
		  AND (c.back LIKE ? OR n.title LIKE ?)
		ORDER BY c.created_at DESC LIMIT 20
	`, "%"+keyword+"%", "%"+keyword+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCards(rows)
}

func scanCards(rows *sql.Rows) ([]*store.DBCard, error) {
	var cards []*store.DBCard
	for rows.Next() {
		c := &store.DBCard{}
		var createdAt string
		if err := rows.Scan(&c.ID, &c.NoteID, &c.SentenceHash, &c.ClozeKey, &c.Kind, &c.Front, &c.Back, &c.Status, &c.Source, &createdAt); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}
