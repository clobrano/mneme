package sync

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/clobrano/mneme/internal/store"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func writeNote(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	return path
}

// TestSyncNeverWritesResolutionLog: acceptance criterion 4
func TestSyncNeverWritesResolutionLog(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()

	writeNote(t, dir, "kubernetes.md", `# Kubernetes
#research

Kubernetes is a container orchestration platform.

Kubernetes uses [[etcd]] for storage.
`)

	_, err := Sync(db, dir)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM resolution_log`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("resolution_log should be empty after sync, got %d entries", count)
	}
}

// TestCosmeticWikiLinkRename: acceptance criterion 2
// Changing wiki-link capitalization (cosmetic: [[etcd]] → [[ETCD]]) normalizes
// to the same sentence_hash AND cloze_key → 0 orphans, card history intact.
func TestCosmeticWikiLinkRename(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()

	notePath := writeNote(t, dir, "kubernetes.md", `# Kubernetes
#research

Kubernetes uses [[etcd]] for storage.
`)

	result1, err := Sync(db, dir)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if result1.New == 0 {
		t.Error("expected new cards on first sync")
	}

	// Change capitalization only — both hash and cloze_key normalize to "etcd"
	if err := os.WriteFile(notePath, []byte(`# Kubernetes
#research

Kubernetes uses [[ETCD]] for storage.
`), 0644); err != nil {
		t.Fatal(err)
	}

	result2, err := Sync(db, dir)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if result2.Orphaned != 0 {
		t.Errorf("cosmetic wiki-link rename should produce 0 orphans, got %d", result2.Orphaned)
	}
}

// TestRewordingOrphans: acceptance criterion 3
// Rewording a fact → 1 orphan + 1 new card
func TestRewordingOrphans(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()

	notePath := writeNote(t, dir, "kubernetes.md", `# Kubernetes
#research

Kubernetes is a container orchestration platform.
`)

	result1, err := Sync(db, dir)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if result1.New == 0 {
		t.Error("expected new cards on first sync")
	}

	// Reword the fact
	if err := os.WriteFile(notePath, []byte(`# Kubernetes
#research

Kubernetes is an orchestration system for containers.
`), 0644); err != nil {
		t.Fatal(err)
	}

	result2, err := Sync(db, dir)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if result2.Orphaned == 0 {
		t.Error("rewording should produce at least 1 orphan")
	}
	if result2.New == 0 {
		t.Error("rewording should produce at least 1 new card")
	}
}

// TestNoteDeleteOrphansCards: acceptance criterion 9
// Deleting a note file → its cards become orphaned, not deleted
func TestNoteDeleteOrphansCards(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()

	notePath := writeNote(t, dir, "kubernetes.md", `# Kubernetes
#research

Kubernetes is a container orchestration platform.
`)

	result1, err := Sync(db, dir)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	newCards := result1.New

	// Delete the note file
	if err := os.Remove(notePath); err != nil {
		t.Fatal(err)
	}

	result2, err := Sync(db, dir)
	if err != nil {
		t.Fatalf("second sync after deletion: %v", err)
	}
	if result2.Orphaned != newCards {
		t.Errorf("deleted note should orphan %d cards, got %d", newCards, result2.Orphaned)
	}

	// Cards should still exist in DB as orphaned, not deleted
	cardRepo := store.NewCardRepository(db)
	orphaned, err := cardRepo.ListOrphaned()
	if err != nil {
		t.Fatal(err)
	}
	if len(orphaned) == 0 {
		t.Error("orphaned cards should be in DB, not deleted")
	}
}

// TestTagUpdatePreservesCardHistory: editing a note's tags should not disturb card learning state
func TestTagUpdatePreservesCardHistory(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()

	notePath := writeNote(t, dir, "kubernetes.md", `# Kubernetes
#research

Kubernetes is a container orchestration platform.
`)

	_, err := Sync(db, dir)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}

	// Give the card some history
	cardRepo := store.NewCardRepository(db)
	reviewRepo := store.NewReviewRepository(db)

	cards, err := cardRepo.ListActiveByNoteID(1)
	if err != nil || len(cards) == 0 {
		// Get note ID first
		noteRepo := store.NewNoteRepository(db)
		n, _ := noteRepo.GetByPath(filepath.Join(dir, "kubernetes.md"))
		if n != nil {
			cards, _ = cardRepo.ListActiveByNoteID(n.ID)
		}
	}
	if len(cards) > 0 {
		rev, _ := reviewRepo.GetByCardID(cards[0].ID)
		if rev != nil {
			rev.Reps = 7
			rev.Lapses = 1
			reviewRepo.Update(rev)
		}
	}

	// Now change only the tag
	if err := os.WriteFile(notePath, []byte(`# Kubernetes
#research
#area/kubernetes

Kubernetes is a container orchestration platform.
`), 0644); err != nil {
		t.Fatal(err)
	}

	result2, err := Sync(db, dir)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if result2.Orphaned != 0 {
		t.Errorf("tag change should produce 0 orphans, got %d", result2.Orphaned)
	}

	// Verify learning state is intact
	if len(cards) > 0 {
		rev, _ := reviewRepo.GetByCardID(cards[0].ID)
		if rev != nil && rev.Reps != 7 {
			t.Errorf("card reps should still be 7, got %d", rev.Reps)
		}
	}
}

// TestSyncIncrementalUnchanged: unchanged notes are skipped
func TestSyncIncrementalUnchanged(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()

	writeNote(t, dir, "kubernetes.md", `# Kubernetes
#research

Kubernetes is a container orchestration platform.
`)

	result1, err := Sync(db, dir)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if result1.Unchanged != 0 {
		t.Errorf("first sync should have 0 unchanged, got %d", result1.Unchanged)
	}

	result2, err := Sync(db, dir)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if result2.Unchanged == 0 {
		t.Error("second sync of unchanged note should report unchanged > 0")
	}
	if result2.New != 0 {
		t.Errorf("second sync should have 0 new, got %d", result2.New)
	}
}
