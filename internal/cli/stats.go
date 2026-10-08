package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"golang.org/x/term"

	"github.com/clobrano/mneme/internal/store"
	"github.com/clobrano/mneme/internal/tui"
)

// RunStats implements the `stats` subcommand.
func RunStats(opts Options, args []string) int {
	db, err := store.Open(opts.Config.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening db: %v\n", err)
		return 1
	}
	defer db.Close()

	if !opts.NoTUI && !opts.JSONOutput && term.IsTerminal(int(os.Stdout.Fd())) {
		if err := tui.RunStatsTUI(db); err != nil {
			fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
			return 1
		}
		return 0
	}

	now := time.Now()
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

	active, err := store.CountActive(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error counting active cards: %v\n", err)
		return 1
	}
	dueToday, err := store.CountDueToday(db, endOfDay)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error counting due today: %v\n", err)
		return 1
	}
	orphaned, err := store.CountOrphaned(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error counting orphans: %v\n", err)
		return 1
	}
	forecast, err := store.DueForecast(db, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting forecast: %v\n", err)
		return 1
	}

	if opts.JSONOutput {
		type forecastItem struct {
			Date string `json:"date"`
			Due  int    `json:"due"`
		}
		var fc []forecastItem
		for _, e := range forecast {
			fc = append(fc, forecastItem{Date: e.Date, Due: e.Count})
		}
		if fc == nil {
			fc = []forecastItem{}
		}
		data := map[string]interface{}{
			"active":    active,
			"due_today": dueToday,
			"forecast":  fc,
			"orphaned":  orphaned,
		}
		b, _ := json.Marshal(data)
		fmt.Println(string(b))
		return 0
	}

	fmt.Printf("Active cards:  %d\n", active)
	fmt.Printf("Due today:     %d\n", dueToday)
	fmt.Printf("Orphaned:      %d\n", orphaned)
	fmt.Println()
	fmt.Println("7-day forecast:")
	if len(forecast) == 0 {
		fmt.Println("  (no cards due in the next 7 days)")
	} else {
		for _, e := range forecast {
			bar := ""
			for i := 0; i < e.Count && i < 40; i++ {
				bar += "█"
			}
			fmt.Printf("  %-12s %3d %s\n", e.Date, e.Count, bar)
		}
	}
	return 0
}
