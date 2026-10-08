package store

import (
	"testing"
	"time"
)

func TestNoteUpsert(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	noteRepo := NewNoteRepository(db)

	id, err := noteRepo.Upsert("/notes/kubernetes.md", "Kubernetes", "abc123")
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}

	note, err := noteRepo.GetByPath("/notes/kubernetes.md")
	if err != nil {
		t.Fatalf("GetByPath: %v", err)
	}
	if note == nil {
		t.Fatal("expected note, got nil")
	}
	if note.Title != "Kubernetes" {
		t.Errorf("Title = %q, want Kubernetes", note.Title)
	}

	id2, err := noteRepo.Upsert("/notes/kubernetes.md", "Kubernetes Updated", "def456")
	if err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	if id2 == 0 {
		t.Error("expected non-zero ID on update")
	}

	note2, err := noteRepo.GetByPath("/notes/kubernetes.md")
	if err != nil {
		t.Fatal(err)
	}
	if note2.ContentHash != "def456" {
		t.Errorf("ContentHash = %q, want def456", note2.ContentHash)
	}
}

func TestNoteListAll(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	noteRepo := NewNoteRepository(db)
	noteRepo.Upsert("/notes/a.md", "A", "hash1")
	noteRepo.Upsert("/notes/b.md", "B", "hash2")

	notes, err := noteRepo.ListAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Errorf("ListAll = %d, want 2", len(notes))
	}
}

func TestTagReplace(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	noteRepo := NewNoteRepository(db)
	tagRepo := NewNoteTagRepository(db)

	noteID, _ := noteRepo.Upsert("/notes/a.md", "A", "hash1")

	if err := tagRepo.Replace(noteID, []string{"research", "study"}); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	tags, err := tagRepo.GetTagsByNoteID(noteID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Errorf("tags = %v, want 2", tags)
	}

	if err := tagRepo.Replace(noteID, []string{"kubernetes"}); err != nil {
		t.Fatal(err)
	}

	tags2, err := tagRepo.GetTagsByNoteID(noteID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags2) != 1 || tags2[0] != "kubernetes" {
		t.Errorf("tags after replace = %v, want [kubernetes]", tags2)
	}
}

func TestCardUniqueConstraint(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	noteRepo := NewNoteRepository(db)
	cardRepo := NewCardRepository(db)

	noteID, _ := noteRepo.Upsert("/notes/test.md", "Test", "hash1")

	id1, isNew1, err := cardRepo.Upsert(noteID, "hash1", "", "definition", "front1", "back1", "template")
	if err != nil {
		t.Fatalf("Upsert 1: %v", err)
	}
	if !isNew1 {
		t.Error("first upsert should be new")
	}

	id2, isNew2, err := cardRepo.Upsert(noteID, "hash1", "", "definition", "front1", "back1", "template")
	if err != nil {
		t.Fatalf("Upsert 2: %v", err)
	}
	if isNew2 {
		t.Error("second upsert of same card should not be new")
	}
	if id1 != id2 {
		t.Errorf("IDs should match: %d vs %d", id1, id2)
	}
}

func TestCardSetStatus(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	noteRepo := NewNoteRepository(db)
	cardRepo := NewCardRepository(db)

	noteID, _ := noteRepo.Upsert("/notes/test.md", "Test", "hash1")
	cardID, _, _ := cardRepo.Upsert(noteID, "hash1", "", "definition", "front", "back", "template")

	if err := cardRepo.SetStatus(cardID, "orphaned"); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	orphaned, err := cardRepo.ListOrphaned()
	if err != nil {
		t.Fatal(err)
	}
	if len(orphaned) != 1 {
		t.Errorf("orphaned = %d, want 1", len(orphaned))
	}
}

func TestReviewCreateAndGet(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	noteRepo := NewNoteRepository(db)
	cardRepo := NewCardRepository(db)
	reviewRepo := NewReviewRepository(db)

	noteID, _ := noteRepo.Upsert("/notes/test.md", "Test", "hash1")
	cardID, _, _ := cardRepo.Upsert(noteID, "hash1", "", "definition", "front", "back", "template")

	if err := reviewRepo.Create(cardID); err != nil {
		t.Fatalf("Create review: %v", err)
	}

	rev, err := reviewRepo.GetByCardID(cardID)
	if err != nil {
		t.Fatal(err)
	}
	if rev == nil {
		t.Fatal("expected review, got nil")
	}
	if rev.State != "new" {
		t.Errorf("State = %q, want new", rev.State)
	}
	if rev.Reps != 0 {
		t.Errorf("Reps = %d, want 0", rev.Reps)
	}
}

func TestResolutionLog(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	noteRepo := NewNoteRepository(db)
	cardRepo := NewCardRepository(db)
	reviewRepo := NewReviewRepository(db)
	resRepo := NewResolutionLogRepository(db)

	noteID, _ := noteRepo.Upsert("/notes/test.md", "Test", "hash1")
	orphanID, _, _ := cardRepo.Upsert(noteID, "hash_orphan", "", "definition", "front1", "back1", "template")
	targetID, _, _ := cardRepo.Upsert(noteID, "hash_target", "", "definition", "front2", "back2", "template")

	reviewRepo.Create(orphanID)
	reviewRepo.Create(targetID)

	cardRepo.SetStatus(orphanID, "orphaned")

	batchID := "testbatch001"
	if err := resRepo.Insert(batchID, "remap", orphanID, &targetID); err != nil {
		t.Fatalf("Insert resolution: %v", err)
	}

	entries, err := resRepo.GetLastBatch()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Action != "remap" {
		t.Errorf("action = %q, want remap", entries[0].Action)
	}
	if entries[0].OrphanCardID != orphanID {
		t.Errorf("OrphanCardID = %d, want %d", entries[0].OrphanCardID, orphanID)
	}
}

func TestRemapTransaction(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	noteRepo := NewNoteRepository(db)
	cardRepo := NewCardRepository(db)
	reviewRepo := NewReviewRepository(db)

	noteID, _ := noteRepo.Upsert("/notes/test.md", "Test", "hash1")
	orphanID, _, _ := cardRepo.Upsert(noteID, "hash_orphan", "", "definition", "front1", "back1", "template")
	targetID, _, _ := cardRepo.Upsert(noteID, "hash_target", "", "definition", "front2", "back2", "template")

	reviewRepo.Create(orphanID)
	reviewRepo.Create(targetID)
	cardRepo.SetStatus(orphanID, "orphaned")

	rev, _ := reviewRepo.GetByCardID(orphanID)
	rev.Reps = 5
	rev.Lapses = 1
	rev.Stability = 3.5
	rev.State = "review"
	rev.Due = time.Now().Add(24 * time.Hour)
	reviewRepo.Update(rev)

	if err := Remap(db, orphanID, targetID, "batch1"); err != nil {
		t.Fatalf("Remap: %v", err)
	}

	orphans, _ := cardRepo.ListOrphaned()
	for _, c := range orphans {
		if c.ID == orphanID {
			t.Error("orphan should be superseded, not orphaned")
		}
	}

	targetRev, _ := reviewRepo.GetByCardID(targetID)
	if targetRev == nil {
		t.Fatal("target should have review row")
	}
	if targetRev.Reps != 5 {
		t.Errorf("target reps = %d, want 5", targetRev.Reps)
	}
}

func TestUndoRemap(t *testing.T) {
	db, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	noteRepo := NewNoteRepository(db)
	cardRepo := NewCardRepository(db)
	reviewRepo := NewReviewRepository(db)

	noteID, _ := noteRepo.Upsert("/notes/test.md", "Test", "hash1")
	orphanID, _, _ := cardRepo.Upsert(noteID, "hash_orphan", "", "definition", "front1", "back1", "template")
	targetID, _, _ := cardRepo.Upsert(noteID, "hash_target", "", "definition", "front2", "back2", "template")

	reviewRepo.Create(orphanID)
	reviewRepo.Create(targetID)
	cardRepo.SetStatus(orphanID, "orphaned")

	rev, _ := reviewRepo.GetByCardID(orphanID)
	rev.Reps = 3
	rev.Lapses = 0
	rev.State = "review"
	rev.Due = time.Now().Add(48 * time.Hour)
	reviewRepo.Update(rev)

	if err := Remap(db, orphanID, targetID, "batch2"); err != nil {
		t.Fatalf("Remap: %v", err)
	}

	if err := UndoLastBatch(db); err != nil {
		t.Fatalf("UndoLastBatch: %v", err)
	}

	orphanCards, _ := cardRepo.queryCards(
		`SELECT id, note_id, sentence_hash, cloze_key, kind, front, back, status, source, created_at FROM cards WHERE id = ?`,
		orphanID,
	)
	if len(orphanCards) == 0 || orphanCards[0].Status != "orphaned" {
		status := "not found"
		if len(orphanCards) > 0 {
			status = orphanCards[0].Status
		}
		t.Errorf("orphan status = %q after undo, want orphaned", status)
	}

	resRepo := NewResolutionLogRepository(db)
	entries, _ := resRepo.GetLastBatch()
	if len(entries) != 0 {
		t.Errorf("resolution_log should be empty after undo, got %d entries", len(entries))
	}
}
