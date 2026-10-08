package generator

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	"github.com/clobrano/mneme/internal/parser"
)

// Card represents a generated flashcard.
type Card struct {
	NoteID       int64
	SentenceHash string
	ClozeKey     string
	Kind         string // definition | cloze | property
	Front        string
	Back         string
	Source       string // always "template" in v1
}

var (
	wikiRe       = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

// SentenceHash computes the canonical hash for a sentence per §6.1.
func SentenceHash(sentence string) string {
	// 1. Trim
	s := strings.TrimSpace(sentence)
	// 2. Replace [[X]] with X
	s = wikiRe.ReplaceAllStringFunc(s, func(m string) string {
		inner := wikiRe.FindStringSubmatch(m)
		if len(inner) > 1 {
			return inner[1]
		}
		return m
	})
	// 3. Lowercase
	s = strings.ToLower(s)
	// 4. Collapse whitespace runs to single space
	s = whitespaceRe.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	// 5. Strip leading and trailing . and ,
	s = strings.Trim(s, ".,")
	s = strings.TrimSpace(s)
	// 6. Return hex(sha256(result))
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

// ClozeKey returns the normalized cloze key for a wiki-link inner text.
// Returns empty string for definition/property cards.
func ClozeKey(linkText string) string {
	return strings.ToLower(linkText)
}

// GenerateCards generates all cards for a note deterministically.
func GenerateCards(note *parser.Note) []Card {
	var cards []Card

	topic := note.Title
	// Find the first "is"/"are" sentence index
	isAreSentenceIdx := -1
	isAreRe := regexp.MustCompile(`\b(is|are)\b`)
	for i, fact := range note.Facts {
		if isAreRe.MatchString(fact) {
			isAreSentenceIdx = i
			break
		}
	}

	for i, fact := range note.Facts {
		links := parser.ExtractWikiLinks(fact)
		hasIsAre := i == isAreSentenceIdx

		// Generate definition card for the first is/are sentence
		if hasIsAre {
			front := fmt.Sprintf("What is %s?", topic)
			if topic == "" {
				front = "What is it?"
			}
			cards = append(cards, Card{
				NoteID:       note.ID,
				SentenceHash: SentenceHash(fact),
				ClozeKey:     "",
				Kind:         "definition",
				Front:        front,
				Back:         fact,
				Source:       "template",
			})
		}

		// Generate cloze cards for wiki-links
		if len(links) > 0 {
			// Deduplicate links while preserving order
			seen := make(map[string]bool)
			for _, link := range links {
				key := ClozeKey(link)
				if seen[key] {
					continue
				}
				seen[key] = true

				// Replace only THIS link's text with ___, leave others intact
				front := replaceSingleLink(fact, link)

				cards = append(cards, Card{
					NoteID:       note.ID,
					SentenceHash: SentenceHash(fact),
					ClozeKey:     key,
					Kind:         "cloze",
					Front:        front,
					Back:         fact,
					Source:       "template",
				})
			}
		} else if !hasIsAre {
			// Property card: sentence with no is/are and no wiki-links
			front := makePropertyFront(fact, topic)
			cards = append(cards, Card{
				NoteID:       note.ID,
				SentenceHash: SentenceHash(fact),
				ClozeKey:     "",
				Kind:         "property",
				Front:        front,
				Back:         fact,
				Source:       "template",
			})
		}
	}

	return cards
}

// replaceSingleLink replaces [[linkText]] with ___ in the sentence (first occurrence only).
// Other wiki-links are left intact.
func replaceSingleLink(sentence, linkText string) string {
	target := "[[" + linkText + "]]"
	idx := strings.Index(sentence, target)
	if idx == -1 {
		return sentence
	}
	return sentence[:idx] + "___" + sentence[idx+len(target):]
}

// makePropertyFront produces the front for a property card by replacing
// the predicate (everything after TOPIC + first token) with ___.
func makePropertyFront(fact, topic string) string {
	if topic == "" {
		return "___"
	}

	lower := strings.ToLower(fact)
	lowerTopic := strings.ToLower(topic)

	// Check fact starts with topic
	if !strings.HasPrefix(lower, lowerTopic) {
		// Can't build proper front; blank it
		return fact
	}

	// Remove topic prefix
	rest := strings.TrimSpace(fact[len(topic):])

	// Find first whitespace after first token
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return fact
	}

	firstToken := fields[0]
	// Find where firstToken ends in rest
	tokenEnd := strings.Index(rest, firstToken) + len(firstToken)
	prefix := fact[:len(topic)] + " " + rest[:tokenEnd]
	return strings.TrimSpace(prefix) + " ___"
}
