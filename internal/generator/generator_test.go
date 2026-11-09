package generator

import (
	"errors"
	"testing"

	"github.com/clobrano/mneme/internal/parser"
)

func makeNote(title string, facts []string) *parser.Note {
	return &parser.Note{Title: title, Facts: facts}
}

func oneCard(t *testing.T, topic, fact string) Card {
	t.Helper()
	cards, err := FactCards(topic, fact)
	if err != nil {
		t.Fatalf("FactCards(%q): %v", fact, err)
	}
	if len(cards) != 1 {
		t.Fatalf("FactCards(%q) = %d cards, want 1: %+v", fact, len(cards), cards)
	}
	return cards[0]
}

func TestMarkerCloze(t *testing.T) {
	c := oneCard(t, "This concept", "This concept is a way to build :: some random thing.")
	if c.Kind != KindCloze {
		t.Errorf("kind = %q, want cloze", c.Kind)
	}
	if want := "This concept is a way to build ___."; c.Front != want {
		t.Errorf("front = %q, want %q", c.Front, want)
	}
	if want := "This concept is a way to build some random thing."; c.Back != want {
		t.Errorf("back = %q, want %q", c.Back, want)
	}
	if c.ClozeKey != "some random thing" {
		t.Errorf("cloze key = %q", c.ClozeKey)
	}
}

func TestBoldCloze(t *testing.T) {
	c := oneCard(t, "This concept", "This concept is a way to build **some random thing**.")
	if c.Kind != KindCloze {
		t.Errorf("kind = %q, want cloze", c.Kind)
	}
	if want := "This concept is a way to build ___."; c.Front != want {
		t.Errorf("front = %q, want %q", c.Front, want)
	}
	if want := "This concept is a way to build some random thing."; c.Back != want {
		t.Errorf("back = %q, want %q", c.Back, want)
	}
}

func TestBoldMidSentence(t *testing.T) {
	c := oneCard(t, "Kubernetes", "Kubernetes uses **[[etcd]]** for persistent storage.")
	if want := "Kubernetes uses ___ for persistent storage."; c.Front != want {
		t.Errorf("front = %q, want %q", c.Front, want)
	}
	if want := "Kubernetes uses [[etcd]] for persistent storage."; c.Back != want {
		t.Errorf("back = %q, want %q", c.Back, want)
	}
	if c.ClozeKey != "etcd" {
		t.Errorf("cloze key = %q, want etcd", c.ClozeKey)
	}
}

func TestMultipleBoldSpans(t *testing.T) {
	cards, err := FactCards("Kubernetes", "Kubernetes uses **etcd** for storage and **CNI** for networking.")
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	// Each card blanks only its own span; the other is shown unwrapped.
	if want := "Kubernetes uses ___ for storage and CNI for networking."; cards[0].Front != want {
		t.Errorf("front[0] = %q, want %q", cards[0].Front, want)
	}
	if want := "Kubernetes uses etcd for storage and ___ for networking."; cards[1].Front != want {
		t.Errorf("front[1] = %q, want %q", cards[1].Front, want)
	}
	if cards[0].SentenceHash != cards[1].SentenceHash {
		t.Error("cards from the same fact must share the sentence hash")
	}
}

func TestDuplicateBoldDeduplicated(t *testing.T) {
	cards, _ := FactCards("K", "K uses **etcd** and **ETCD**.")
	if len(cards) != 1 {
		t.Errorf("got %d cards, want 1 (same answer deduplicated)", len(cards))
	}
}

func TestMarkerAndBoldTogether(t *testing.T) {
	cards, err := FactCards("Kubernetes", "Kubernetes keeps **cluster state** in :: etcd.")
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	if want := "Kubernetes keeps cluster state in ___."; cards[0].Front != want {
		t.Errorf(":: front = %q, want %q", cards[0].Front, want)
	}
	if want := "Kubernetes keeps ___ in etcd."; cards[1].Front != want {
		t.Errorf("bold front = %q, want %q", cards[1].Front, want)
	}
}

func TestQuestionCard(t *testing.T) {
	c := oneCard(t, "Kubernetes", "Which port does the Kubernetes API server use? :: 6443")
	if c.Kind != KindQuestion {
		t.Errorf("kind = %q, want question", c.Kind)
	}
	if c.Front != "Which port does the Kubernetes API server use?" || c.Back != "6443" {
		t.Errorf("front/back = %q / %q", c.Front, c.Back)
	}
}

func TestDefinitionCard(t *testing.T) {
	c := oneCard(t, "ETCD", "ETCD is :: a distributed key-value store.")
	if c.Kind != KindDefinition {
		t.Errorf("kind = %q, want definition", c.Kind)
	}
	if c.Front != "What is ETCD?" {
		t.Errorf("front = %q", c.Front)
	}
	if c.Back != "ETCD is a distributed key-value store." {
		t.Errorf("back = %q", c.Back)
	}

	c = oneCard(t, "Pods", "Pods are :: the smallest deployable units.")
	if c.Front != "What are Pods?" {
		t.Errorf("plural front = %q", c.Front)
	}
}

// Only an exact "TOPIC is ::" cue is a definition; anything longer is a cloze.
func TestIsInsideCueIsCloze(t *testing.T) {
	c := oneCard(t, "Kubernetes", "Kubernetes is written in :: Go.")
	if c.Kind != KindCloze || c.Front != "Kubernetes is written in ___." {
		t.Errorf("got %q %q", c.Kind, c.Front)
	}
}

func TestMarkersInsideCodeIgnored(t *testing.T) {
	_, err := FactCards("C++", "C++ uses `std::vector` and `**ptr` for things.")
	if !errors.Is(err, ErrNoMarker) {
		t.Errorf("err = %v, want ErrNoMarker", err)
	}

	c := oneCard(t, "C++", "C++ declares dynamic arrays with :: `std::vector`.")
	if c.Front != "C++ declares dynamic arrays with ___." {
		t.Errorf("front = %q", c.Front)
	}
}

func TestNoCardErrors(t *testing.T) {
	cases := map[string]error{
		"Kubernetes supports horizontal scaling.": ErrNoMarker,
		"Kubernetes uses :: etcd :: for storage.": ErrMultipleMarkers,
		":: etcd":                                 ErrEmptyCue,
		"Kubernetes uses ::":                      ErrEmptyAnswer,
	}
	for fact, want := range cases {
		if _, err := FactCards("Kubernetes", fact); !errors.Is(err, want) {
			t.Errorf("FactCards(%q) err = %v, want %v", fact, err, want)
		}
	}
}

func TestGenerateCardsSkipsUnmarkedFacts(t *testing.T) {
	note := makeNote("Kubernetes", []string{
		"Kubernetes is :: a container orchestration platform.",
		"Kubernetes supports horizontal scaling.",
		"Kubernetes uses **etcd** for storage.",
	})
	note.ID = 42
	cards := GenerateCards(note)
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	for _, c := range cards {
		if c.NoteID != 42 || c.Source != "template" {
			t.Errorf("card %+v: bad NoteID/Source", c)
		}
	}
}

func TestDeterministic(t *testing.T) {
	note := makeNote("Kubernetes", []string{
		"Kubernetes is :: a container orchestration platform.",
		"Kubernetes uses **etcd** for storage and **CNI** for networking.",
	})
	a, b := GenerateCards(note), GenerateCards(note)
	if len(a) != len(b) {
		t.Fatalf("len %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("card[%d] differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// Changing which part is the answer keeps the sentence identity, so history
// can be transferred via resolve; only the cloze key changes.
func TestMovingMarkersKeepsSentenceHash(t *testing.T) {
	variants := []string{
		"Kubernetes uses **etcd** for storage.",
		"Kubernetes uses etcd for **storage**.",
		"Kubernetes uses etcd for :: storage.",
		"Kubernetes uses :: etcd for storage.",
	}
	var hash string
	for _, v := range variants {
		cards, err := FactCards("Kubernetes", v)
		if err != nil {
			t.Fatal(err)
		}
		if hash == "" {
			hash = cards[0].SentenceHash
		} else if cards[0].SentenceHash != hash {
			t.Errorf("%q: sentence hash changed", v)
		}
	}
}

func TestCosmeticEditPreservesHash(t *testing.T) {
	if SentenceHash("Kubernetes uses [[etcd]] for storage.") != SentenceHash("Kubernetes uses etcd for storage.") {
		t.Error("hash changed after wiki-link rename")
	}
}

func TestTrailingPunctuationPreservesHash(t *testing.T) {
	if SentenceHash("Kubernetes is a container orchestrator.") != SentenceHash("Kubernetes is a container orchestrator") {
		t.Error("hash changed after removing trailing period")
	}
}

func TestRewordingChangesHash(t *testing.T) {
	if SentenceHash("Kubernetes is a container orchestration platform.") == SentenceHash("Kubernetes is an orchestration system for containers.") {
		t.Error("hash should differ after rewording")
	}
}

func TestSentenceHashNormalization(t *testing.T) {
	h1 := SentenceHash("Kubernetes IS a platform.")
	if h1 != SentenceHash("kubernetes is a platform.") {
		t.Error("hash should be case-insensitive")
	}
	if h1 != SentenceHash("Kubernetes  is  a  platform.") {
		t.Error("hash should collapse whitespace")
	}
}

func TestClozeKey(t *testing.T) {
	cases := map[string]string{
		"CNI":                   "cni",
		"":                      "",
		"[[etcd]].":             "etcd",
		"Some   Random thing. ": "some random thing",
	}
	for in, want := range cases {
		if got := ClozeKey(in); got != want {
			t.Errorf("ClozeKey(%q) = %q, want %q", in, got, want)
		}
	}
}
