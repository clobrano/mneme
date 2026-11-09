package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/clobrano/mneme/internal/generator"
	"github.com/clobrano/mneme/internal/parser"
)

// RunGenerate implements the `generate` subcommand.
func RunGenerate(opts Options, args []string) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	showUnmarked := fs.Bool("unmarked", false, "list facts that have no '::' or **bold** marker")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	notesDir := opts.Config.NotesDir
	if notesDir == "" {
		fmt.Fprintln(os.Stderr, "error: notes_dir is not configured")
		return 1
	}

	var allCards []map[string]interface{}
	var unmarked []string

	err := filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		note, err := parser.ParseNote(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: parse error %s: %v\n", path, err)
			return nil
		}
		for _, fact := range note.Facts {
			cards, err := generator.FactCards(note.Title, fact)
			if errors.Is(err, generator.ErrNoMarker) {
				unmarked = append(unmarked, fmt.Sprintf("%s: %s", filepath.Base(path), fact))
				continue
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: %s: %v: %s\n", filepath.Base(path), err, fact)
				continue
			}
			for _, c := range cards {
				allCards = append(allCards, map[string]interface{}{
					"note":          filepath.Base(path),
					"kind":          c.Kind,
					"front":         c.Front,
					"back":          c.Back,
					"sentence_hash": c.SentenceHash,
					"cloze_key":     c.ClozeKey,
				})
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error walking notes dir: %v\n", err)
		return 1
	}

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(allCards); err != nil {
			fmt.Fprintf(os.Stderr, "json encode: %v\n", err)
			return 1
		}
		reportUnmarked(unmarked, *showUnmarked)
		return 0
	}
	defer reportUnmarked(unmarked, *showUnmarked)

	if len(allCards) == 0 {
		fmt.Println("No cards generated.")
		return 0
	}

	fmt.Printf("%-12s %-10s %s\n", "KIND", "HASH", "FRONT")
	fmt.Printf("%-12s %-10s %s\n", "----", "----", "-----")
	for _, c := range allCards {
		hash := fmt.Sprintf("%v", c["sentence_hash"])
		if len(hash) > 8 {
			hash = hash[:8]
		}
		front := fmt.Sprintf("%v", c["front"])
		if len(front) > 60 {
			front = front[:57] + "..."
		}
		fmt.Printf("%-12s %-10s %s\n", c["kind"], hash, front)
	}
	return 0
}

// reportUnmarked prints, to stderr, how many facts produced no card because
// they carry no marker, and lists them when asked to.
func reportUnmarked(unmarked []string, list bool) {
	if len(unmarked) == 0 {
		return
	}
	if !list {
		fmt.Fprintf(os.Stderr, "\n%d fact(s) without '::' or **bold** produced no card (list them with --unmarked)\n", len(unmarked))
		return
	}
	fmt.Fprintf(os.Stderr, "\nFacts without '::' or **bold** (no card):\n")
	for _, u := range unmarked {
		fmt.Fprintf(os.Stderr, "  %s\n", u)
	}
}
