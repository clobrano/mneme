package sync

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"

	"github.com/clobrano/mneme/internal/generator"
	"github.com/clobrano/mneme/internal/parser"
	"github.com/clobrano/mneme/internal/store"
)

// NoteFile holds a path and its content hash.
type NoteFile struct {
	Path        string
	ContentHash string
}

// SyncResult summarises what happened during a sync.
type SyncResult struct {
	Unchanged int
	New       int
	Orphaned  int
}

// ScanNotes walks notesDir and returns all .md files with their content hashes.
func ScanNotes(notesDir string) ([]NoteFile, error) {
	var files []NoteFile
	err := filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		hash, err := hashFile(path)
		if err != nil {
			return fmt.Errorf("hashing %s: %w", path, err)
		}
		files = append(files, NoteFile{Path: path, ContentHash: hash})
		return nil
	})
	return files, err
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Sync performs an incremental reindex of the notes directory.
// It MUST NEVER insert into resolution_log or perform any remap/discard.
func Sync(db *sql.DB, notesDir string) (SyncResult, error) {
	noteFiles, err := ScanNotes(notesDir)
	if err != nil {
		return SyncResult{}, fmt.Errorf("scanning notes: %w", err)
	}

	noteRepo := store.NewNoteRepository(db)
	tagRepo := store.NewNoteTagRepository(db)
	cardRepo := store.NewCardRepository(db)
	reviewRepo := store.NewReviewRepository(db)

	// Build set of paths found on disk
	diskPaths := make(map[string]bool)
	for _, nf := range noteFiles {
		diskPaths[nf.Path] = true
	}

	var result SyncResult

	// Track which (noteID, sentenceHash, clozeKey) tuples were regenerated
	type cardKey struct {
		noteID       int64
		sentenceHash string
		clozeKey     string
	}
	regenerated := make(map[cardKey]bool)

	for _, nf := range noteFiles {
		existing, err := noteRepo.GetByPath(nf.Path)
		if err != nil {
			return result, fmt.Errorf("get note %s: %w", nf.Path, err)
		}

		if existing != nil && existing.ContentHash == nf.ContentHash {
			// Unchanged — still track its cards as regenerated
			activeCards, err := cardRepo.ListActiveByNoteID(existing.ID)
			if err != nil {
				return result, err
			}
			for _, c := range activeCards {
				regenerated[cardKey{existing.ID, c.SentenceHash, c.ClozeKey}] = true
			}
			result.Unchanged++
			continue
		}

		// New or changed — parse and index
		note, err := parser.ParseNote(nf.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: parse error %s: %v\n", nf.Path, err)
			continue
		}

		noteID, err := noteRepo.Upsert(nf.Path, note.Title, nf.ContentHash)
		if err != nil {
			return result, fmt.Errorf("upsert note %s: %w", nf.Path, err)
		}
		note.ID = noteID

		if err := tagRepo.Replace(noteID, note.Tags); err != nil {
			return result, fmt.Errorf("replace tags for %s: %w", nf.Path, err)
		}

		cards := generator.GenerateCards(note)
		for _, gc := range cards {
			cardID, isNew, err := cardRepo.Upsert(noteID, gc.SentenceHash, gc.ClozeKey, gc.Kind, gc.Front, gc.Back, gc.Source)
			if err != nil {
				return result, fmt.Errorf("upsert card: %w", err)
			}
			if isNew {
				if err := reviewRepo.Create(cardID); err != nil {
					return result, fmt.Errorf("create review: %w", err)
				}
				result.New++
			}
			regenerated[cardKey{noteID, gc.SentenceHash, gc.ClozeKey}] = true
		}
	}

	// Orphan detection: active cards not in regenerated set for live notes
	allNotes, err := noteRepo.ListAll()
	if err != nil {
		return result, fmt.Errorf("list all notes: %w", err)
	}

	for _, dbNote := range allNotes {
		if !diskPaths[dbNote.Path] {
			// Deleted note: orphan all its active cards
			activeCards, err := cardRepo.ListActiveByNoteID(dbNote.ID)
			if err != nil {
				return result, err
			}
			for _, c := range activeCards {
				if err := cardRepo.SetStatus(c.ID, "orphaned"); err != nil {
					return result, err
				}
				result.Orphaned++
			}
			continue
		}

		// Live note: orphan cards that weren't regenerated
		activeCards, err := cardRepo.ListActiveByNoteID(dbNote.ID)
		if err != nil {
			return result, err
		}
		for _, c := range activeCards {
			key := cardKey{dbNote.ID, c.SentenceHash, c.ClozeKey}
			if !regenerated[key] {
				if err := cardRepo.SetStatus(c.ID, "orphaned"); err != nil {
					return result, err
				}
				result.Orphaned++
			}
		}
	}

	return result, nil
}

// Watch watches notesDir for changes and re-runs Sync on any .md file change.
// It calls onSync after each sync with the result. Returns when stopCh is closed.
func Watch(db *sql.DB, notesDir string, onSync func(SyncResult, error)) (func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating watcher: %w", err)
	}

	if err := watcher.Add(notesDir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("watching %s: %w", notesDir, err)
	}

	stop := make(chan struct{})
	go func() {
		defer watcher.Close()
		for {
			select {
			case <-stop:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Ext(event.Name) == ".md" {
					result, err := Sync(db, notesDir)
					onSync(result, err)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)
			}
		}
	}()

	return func() { close(stop) }, nil
}
