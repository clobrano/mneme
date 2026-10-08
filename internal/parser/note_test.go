package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func writeNote(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.md")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTitleExtraction(t *testing.T) {
	p := writeNote(t, "# Kubernetes\n#research\n\nKubernetes is a container orchestrator.\n")
	note, err := ParseNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if note.Title != "Kubernetes" {
		t.Errorf("Title = %q, want %q", note.Title, "Kubernetes")
	}
}

func TestSingleTagLine(t *testing.T) {
	p := writeNote(t, "# Kubernetes\n#research\n\nKubernetes is a container orchestrator.\n")
	note, err := ParseNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Tags) != 1 || note.Tags[0] != "research" {
		t.Errorf("Tags = %v, want [research]", note.Tags)
	}
}

func TestMultiTagLine(t *testing.T) {
	p := writeNote(t, "# Kubernetes\n#area/kubernetes #cka\n\nKubernetes is a container orchestrator.\n")
	note, err := ParseNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Tags) != 2 {
		t.Fatalf("Tags = %v, want 2 elements", note.Tags)
	}
	if note.Tags[0] != "area/kubernetes" {
		t.Errorf("Tags[0] = %q, want %q", note.Tags[0], "area/kubernetes")
	}
	if note.Tags[1] != "cka" {
		t.Errorf("Tags[1] = %q, want %q", note.Tags[1], "cka")
	}
}

func TestHierarchicalTag(t *testing.T) {
	p := writeNote(t, "# Topic\n#area/networking\n\nTopic is a thing.\n")
	note, err := ParseNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Tags) != 1 || note.Tags[0] != "area/networking" {
		t.Errorf("Tags = %v, want [area/networking]", note.Tags)
	}
}

func TestMultipleTagLines(t *testing.T) {
	p := writeNote(t, "# Topic\n#research\n#study\n\nTopic is a thing.\n")
	note, err := ParseNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Tags) != 2 {
		t.Fatalf("Tags = %v, want 2 elements", note.Tags)
	}
}

func TestTagsLowercased(t *testing.T) {
	p := writeNote(t, "# Topic\n#Research\n\nTopic is a thing.\n")
	note, err := ParseNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Tags) != 1 || note.Tags[0] != "research" {
		t.Errorf("Tags = %v, expected lowercase", note.Tags)
	}
}

func TestFactSplittingOnBlankLines(t *testing.T) {
	content := `# Topic
#research

Topic is a definition.

Topic has property A.

Topic has property B.
`
	p := writeNote(t, content)
	note, err := ParseNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Facts) != 3 {
		t.Fatalf("Facts = %v (%d), want 3 facts", note.Facts, len(note.Facts))
	}
	if note.Facts[0] != "Topic is a definition." {
		t.Errorf("Facts[0] = %q", note.Facts[0])
	}
}

func TestWikiLinkExtraction(t *testing.T) {
	sentence := "Kubernetes uses [[etcd]] for storage and [[CNI]] for networking."
	links := ExtractWikiLinks(sentence)
	if len(links) != 2 {
		t.Fatalf("ExtractWikiLinks = %v, want 2 links", links)
	}
	if links[0] != "etcd" {
		t.Errorf("links[0] = %q, want %q", links[0], "etcd")
	}
	if links[1] != "CNI" {
		t.Errorf("links[1] = %q, want %q", links[1], "CNI")
	}
}

func TestWikiLinkExtractionEmpty(t *testing.T) {
	links := ExtractWikiLinks("No links here.")
	if len(links) != 0 {
		t.Errorf("ExtractWikiLinks = %v, want empty", links)
	}
}

func TestWarnFactMissingTopic(t *testing.T) {
	content := "# Topic\n\nSomething else is not starting with topic.\n"
	p := writeNote(t, content)

	// Capture stderr
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	note, err := ParseNote(p)
	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])

	if err != nil {
		t.Fatalf("ParseNote should not error on format issues: %v", err)
	}
	if note == nil {
		t.Fatal("note should not be nil")
	}
	if len(note.Facts) == 0 {
		t.Error("fact should still be parsed even if it doesn't start with topic")
	}
	if stderr == "" {
		t.Error("expected warning on stderr for fact not starting with topic")
	}
}

func TestNoTitle(t *testing.T) {
	content := "Just some content\nwithout a title.\n"
	p := writeNote(t, content)
	note, err := ParseNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if note.Title != "" {
		t.Errorf("Title should be empty without # header, got %q", note.Title)
	}
}

func TestMultipleBlankLinesBetweenFacts(t *testing.T) {
	content := "# Topic\n\nTopic is a definition.\n\n\nTopic has property.\n"
	p := writeNote(t, content)
	note, err := ParseNote(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Facts) != 2 {
		t.Fatalf("Facts = %v, want 2", note.Facts)
	}
}
