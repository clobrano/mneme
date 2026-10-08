package cli

import (
	"encoding/json"
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
	if err := fs.Parse(args); err != nil {
		return 1
	}

	notesDir := opts.Config.NotesDir
	if notesDir == "" {
		fmt.Fprintln(os.Stderr, "error: notes_dir is not configured")
		return 1
	}

	var allCards []map[string]interface{}

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
		cards := generator.GenerateCards(note)
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
		return 0
	}

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
