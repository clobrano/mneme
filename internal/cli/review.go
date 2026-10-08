package cli

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/clobrano/mneme/internal/scheduler"
	"github.com/clobrano/mneme/internal/store"
	"github.com/clobrano/mneme/internal/tui"
)

// RunReview implements the `review` subcommand.
func RunReview(opts Options, args []string) int {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	var tags multiFlag
	var excludeTags multiFlag
	matchMode := fs.String("match", "", "any|all (default from config)")
	noteTitle := fs.String("note", "", "filter by note title")
	limit := fs.Int("limit", 0, "maximum number of cards to review")
	fs.Var(&tags, "tag", "filter by tag (repeatable)")
	fs.Var(&excludeTags, "exclude-tag", "exclude notes with tag (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Apply config defaults when no CLI tag flags given; CLI flags fully replace config.
	filterTags := []string(tags)
	filterExclude := []string(excludeTags)
	filterMatch := *matchMode

	if len(filterTags) == 0 {
		filterTags = opts.Config.Review.Tags
		filterExclude = opts.Config.Review.Exclude
		filterMatch = opts.Config.Review.Match
	}
	if filterMatch == "" {
		filterMatch = "any"
	}

	db, err := store.Open(opts.Config.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening db: %v\n", err)
		return 1
	}
	defer db.Close()

	// Use TUI when terminal is detected and --no-tui not set
	if !opts.NoTUI && term.IsTerminal(int(os.Stdout.Fd())) {
		return runReviewTUI(db, filterTags, filterMatch, filterExclude, *noteTitle, *limit)
	}
	return reviewPlain(db, filterTags, filterMatch, filterExclude, *noteTitle, *limit)
}

func runReviewTUI(db *sql.DB, filterTags []string, filterMatch string, filterExclude []string, noteTitle string, limit int) int {
	now := time.Now()
	cards, err := store.DueCards(db, now, filterTags, filterMatch, filterExclude, noteTitle, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching due cards: %v\n", err)
		return 1
	}
	if err := tui.RunReviewTUI(db, cards); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		return 1
	}
	return 0
}

func reviewPlain(db *sql.DB, filterTags []string, filterMatch string, filterExclude []string, noteTitle string, limit int) int {
	now := time.Now()
	cards, err := store.DueCards(db, now, filterTags, filterMatch, filterExclude, noteTitle, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching due cards: %v\n", err)
		return 1
	}

	if len(cards) == 0 {
		fmt.Println("No cards due. Great job!")
		return 0
	}

	reviewRepo := store.NewReviewRepository(db)
	logRepo := store.NewReviewLogRepository(db)
	reader := bufio.NewReader(os.Stdin)

	total := len(cards)
	for i, card := range cards {
		fmt.Printf("\n--- Card %d of %d ---\n", i+1, total)
		fmt.Printf("[%s]\n\n", strings.ToUpper(card.Kind))
		fmt.Println(card.Front)
		fmt.Println("\nPress any key to reveal...")

		readAnyKey(reader)

		fmt.Println()
		fmt.Println(card.Back)
		fmt.Println()

		grade := promptGrade(reader)

		rev, err := reviewRepo.GetByCardID(card.ID)
		if err != nil || rev == nil {
			fmt.Fprintf(os.Stderr, "warning: no review row for card %d\n", card.ID)
			continue
		}

		updated := scheduler.Grade(rev, grade, time.Now())
		if err := reviewRepo.Update(updated); err != nil {
			fmt.Fprintf(os.Stderr, "error updating review: %v\n", err)
		}
		if err := logRepo.Insert(card.ID, grade); err != nil {
			fmt.Fprintf(os.Stderr, "error logging review: %v\n", err)
		}
	}

	fmt.Printf("\nSession complete! Reviewed %d card(s).\n", total)
	return 0
}

func readAnyKey(r *bufio.Reader) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		r.ReadString('\n')
		return
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)
	r.ReadByte()
}

func promptGrade(r *bufio.Reader) int {
	for {
		fmt.Print("Grade (1=again  2=hard  3=good  4=easy): ")
		line, _ := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if len(line) == 1 {
			switch line[0] {
			case '1':
				return 1
			case '2':
				return 2
			case '3':
				return 3
			case '4':
				return 4
			}
		}
	}
}

// multiFlag allows a flag to be specified multiple times.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}
