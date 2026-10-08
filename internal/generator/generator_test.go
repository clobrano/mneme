package generator

import (
	"testing"

	"github.com/clobrano/mneme/internal/parser"
)

func makeNote(title string, facts []string) *parser.Note {
	return &parser.Note{
		Title: title,
		Facts: facts,
		ID:    0,
	}
}

// TestDefinitionAndTwoClozeCards: acceptance criterion 1
// "is" sentence + two-link sentence → 1 definition + 2 cloze
func TestDefinitionAndTwoClozeCards(t *testing.T) {
	note := makeNote("Kubernetes", []string{
		"Kubernetes is a container orchestration platform.",
		"Kubernetes uses [[etcd]] for storage and [[CNI]] for networking.",
	})

	cards := GenerateCards(note)

	var defs, clozes, props int
	for _, c := range cards {
		switch c.Kind {
		case "definition":
			defs++
		case "cloze":
			clozes++
		case "property":
			props++
		}
	}

	if defs != 1 {
		t.Errorf("definition cards = %d, want 1", defs)
	}
	if clozes != 2 {
		t.Errorf("cloze cards = %d, want 2", clozes)
	}
	if props != 0 {
		t.Errorf("property cards = %d, want 0", props)
	}
	if len(cards) != 3 {
		t.Errorf("total cards = %d, want 3", len(cards))
	}
}

// TestDeterministic: acceptance criterion 1 - same note → same cards
func TestDeterministic(t *testing.T) {
	note := makeNote("Kubernetes", []string{
		"Kubernetes is a container orchestration platform.",
		"Kubernetes uses [[etcd]] for storage and [[CNI]] for networking.",
	})

	cards1 := GenerateCards(note)
	cards2 := GenerateCards(note)

	if len(cards1) != len(cards2) {
		t.Fatalf("cards1 len=%d, cards2 len=%d", len(cards1), len(cards2))
	}
	for i := range cards1 {
		if cards1[i].SentenceHash != cards2[i].SentenceHash {
			t.Errorf("card[%d] hash differs between runs", i)
		}
		if cards1[i].Front != cards2[i].Front {
			t.Errorf("card[%d] front differs between runs", i)
		}
		if cards1[i].Kind != cards2[i].Kind {
			t.Errorf("card[%d] kind differs between runs", i)
		}
	}
}

// TestCosmeticEditPreservesHash: acceptance criterion 2
// Renaming a wiki-link → same hash
func TestCosmeticEditPreservesHash(t *testing.T) {
	fact1 := "Kubernetes uses [[etcd]] for storage."
	fact2 := "Kubernetes uses etcd for storage."

	h1 := SentenceHash(fact1)
	h2 := SentenceHash(fact2)

	if h1 != h2 {
		t.Errorf("hash changed after wiki-link rename: %s vs %s", h1, h2)
	}
}

// TestTrailingPunctuationPreservesHash: acceptance criterion 2
func TestTrailingPunctuationPreservesHash(t *testing.T) {
	fact1 := "Kubernetes is a container orchestrator."
	fact2 := "Kubernetes is a container orchestrator"

	h1 := SentenceHash(fact1)
	h2 := SentenceHash(fact2)

	if h1 != h2 {
		t.Errorf("hash changed after removing trailing period: %s vs %s", h1, h2)
	}
}

// TestRewordingChangesHash: acceptance criterion 3
func TestRewordingChangesHash(t *testing.T) {
	fact1 := "Kubernetes is a container orchestration platform."
	fact2 := "Kubernetes is an orchestration system for containers."

	h1 := SentenceHash(fact1)
	h2 := SentenceHash(fact2)

	if h1 == h2 {
		t.Error("hash should differ after rewording")
	}
}

// TestIsWithWikiLinks: is sentence + wiki-links → definition + cloze cards
func TestIsWithWikiLinks(t *testing.T) {
	note := makeNote("CNI", []string{
		"CNI is the [[Container Network Interface]] used in [[Kubernetes]].",
	})

	cards := GenerateCards(note)

	var defs, clozes int
	for _, c := range cards {
		switch c.Kind {
		case "definition":
			defs++
		case "cloze":
			clozes++
		}
	}

	if defs != 1 {
		t.Errorf("definition cards = %d, want 1", defs)
	}
	if clozes != 2 {
		t.Errorf("cloze cards = %d, want 2 (one per link)", clozes)
	}
}

func TestDefinitionCardFront(t *testing.T) {
	note := makeNote("ETCD", []string{
		"ETCD is a distributed key-value store.",
	})

	cards := GenerateCards(note)
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	if cards[0].Kind != "definition" {
		t.Errorf("kind = %q, want definition", cards[0].Kind)
	}
	if cards[0].Front != "What is ETCD?" {
		t.Errorf("front = %q, want %q", cards[0].Front, "What is ETCD?")
	}
	if cards[0].Back != "ETCD is a distributed key-value store." {
		t.Errorf("back = %q", cards[0].Back)
	}
}

func TestClozeFront(t *testing.T) {
	note := makeNote("Kubernetes", []string{
		"Kubernetes uses [[etcd]] for storage.",
	})
	cards := GenerateCards(note)
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	if cards[0].Kind != "cloze" {
		t.Errorf("kind = %q, want cloze", cards[0].Kind)
	}
	if cards[0].Front != "Kubernetes uses ___ for storage." {
		t.Errorf("front = %q", cards[0].Front)
	}
}

func TestPropertyCard(t *testing.T) {
	note := makeNote("Kubernetes", []string{
		"Kubernetes supports horizontal scaling automatically.",
	})
	cards := GenerateCards(note)
	if len(cards) != 1 {
		t.Fatalf("expected 1 property card, got %d", len(cards))
	}
	if cards[0].Kind != "property" {
		t.Errorf("kind = %q, want property", cards[0].Kind)
	}
	// Front should have predicate replaced with ___
	if cards[0].Front == cards[0].Back {
		t.Error("property card front should differ from back")
	}
}

func TestSentenceHashNormalization(t *testing.T) {
	// Case insensitive
	h1 := SentenceHash("Kubernetes IS a platform.")
	h2 := SentenceHash("kubernetes is a platform.")
	if h1 != h2 {
		t.Error("hash should be case-insensitive")
	}

	// Extra whitespace
	h3 := SentenceHash("Kubernetes  is  a  platform.")
	if h1 != h3 {
		t.Error("hash should collapse whitespace")
	}
}

func TestClozeKeyLowercase(t *testing.T) {
	if ClozeKey("CNI") != "cni" {
		t.Errorf("ClozeKey(CNI) = %q, want cni", ClozeKey("CNI"))
	}
	if ClozeKey("") != "" {
		t.Errorf("ClozeKey('') should return empty")
	}
}

func TestSourceIsTemplate(t *testing.T) {
	note := makeNote("Topic", []string{
		"Topic is a thing.",
	})
	cards := GenerateCards(note)
	for _, c := range cards {
		if c.Source != "template" {
			t.Errorf("source = %q, want template", c.Source)
		}
	}
}

func TestMultipleClozesSameLink(t *testing.T) {
	// Same link appearing twice should produce only one cloze card (dedup)
	note := makeNote("Kubernetes", []string{
		"Kubernetes uses [[etcd]] and [[etcd]] for storage.",
	})
	cards := GenerateCards(note)
	var clozes int
	for _, c := range cards {
		if c.Kind == "cloze" {
			clozes++
		}
	}
	if clozes != 1 {
		t.Errorf("cloze cards = %d, want 1 (deduplication)", clozes)
	}
}
