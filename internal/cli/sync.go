package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/clobrano/mneme/internal/store"
	mnSync "github.com/clobrano/mneme/internal/sync"
)

// RunSync implements the `sync` subcommand.
func RunSync(opts Options, args []string) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	watch := fs.Bool("watch", false, "watch for changes and re-sync")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	db, err := store.Open(opts.Config.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening db: %v\n", err)
		return 1
	}
	defer db.Close()

	notesDir := opts.Config.NotesDir
	result, err := mnSync.Sync(db, notesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync error: %v\n", err)
		return 1
	}

	printSyncResult(result, opts.JSONOutput)

	if *watch {
		stopFn, err := mnSync.Watch(db, notesDir, func(r mnSync.SyncResult, e error) {
			if e != nil {
				fmt.Fprintf(os.Stderr, "sync error: %v\n", e)
				return
			}
			printSyncResult(r, opts.JSONOutput)
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
			return 1
		}
		defer stopFn()

		fmt.Fprintf(os.Stderr, "Watching %s for changes. Press Ctrl+C to stop.\n", notesDir)
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
	}

	return 0
}

func printSyncResult(result mnSync.SyncResult, jsonOutput bool) {
	if jsonOutput {
		data := map[string]int{
			"unchanged": result.Unchanged,
			"new":       result.New,
			"orphaned":  result.Orphaned,
		}
		b, _ := json.Marshal(data)
		fmt.Println(string(b))
		return
	}

	msg := fmt.Sprintf("%d unchanged, %d new", result.Unchanged, result.New)
	if result.Orphaned > 0 {
		msg += fmt.Sprintf(", %d orphaned — run 'mneme resolve'", result.Orphaned)
	}
	fmt.Println(msg)
}
