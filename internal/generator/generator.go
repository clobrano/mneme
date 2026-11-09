package generator

import (
	"crypto/sha256"
	"errors"
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
	Kind         string // definition | cloze | question
	Front        string
	Back         string
	Source       string // always "template" in v1
}

// Card kinds.
const (
	KindDefinition = "definition"
	KindCloze      = "cloze"
	KindQuestion   = "question"
)

// Reasons a fact produces no card.
var (
	// ErrNoMarker: the fact has neither "::" nor **bold**. Not an error in the
	// note (not every fact needs a card) but reported so the author can spot
	// omissions.
	ErrNoMarker = errors.New("no '::' or **bold** marker")
	// ErrMultipleMarkers: more than one "::" outside code spans.
	ErrMultipleMarkers = errors.New("more than one '::' marker")
	// ErrEmptyCue: nothing before "::".
	ErrEmptyCue = errors.New("nothing before '::'")
	// ErrEmptyAnswer: nothing after "::".
	ErrEmptyAnswer = errors.New("nothing after '::'")
)

var (
	wikiRe       = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	whitespaceRe = regexp.MustCompile(`\s+`)
	trailPunctRe = regexp.MustCompile(`[.!;,]+$`)
)

// SentenceHash computes the canonical hash for a sentence per §6.1.
// Callers pass the sentence with the "::" marker already removed, so moving
// the marker within a sentence does not change the sentence's identity.
func SentenceHash(sentence string) string {
	// 1. Trim
	s := strings.TrimSpace(sentence)
	// 2. Replace [[X]] with X
	s = stripWikiLinks(s)
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

// ClozeKey returns the normalized key of an answer: wiki-links unwrapped,
// lowercased, whitespace collapsed, trailing punctuation removed.
// Together with the sentence hash it identifies a card, so moving the "::"
// marker to a different answer creates a new card.
func ClozeKey(answer string) string {
	s := strings.ToLower(stripWikiLinks(answer))
	s = whitespaceRe.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return strings.TrimSpace(strings.TrimRight(s, ".,;!?"))
}

func stripWikiLinks(s string) string {
	return wikiRe.ReplaceAllString(s, "$1")
}

// FactCards turns one fact into cards. Two markers are recognised, both
// ignored inside `code spans`:
//
//   - **bold**: one cloze card per bold span ("Kubernetes stores state in ___.")
//   - "::" (at most one per fact) splits cue from answer:
//     "Question? :: answer"     → question card (front = the question)
//     "TOPIC is :: definition." → definition card ("What is TOPIC?")
//     "cue :: answer."          → cloze card ("cue ___.")
//
// The markers are removed from the card text. It returns an error (one of the
// Err* values) when the fact yields no card.
func FactCards(topic, fact string) ([]Card, error) {
	cueRaw, answerRaw, n := parser.SplitMarker(fact)
	bold := parser.BoldSpans(fact)
	switch {
	case n > 1:
		return nil, ErrMultipleMarkers
	case n == 0 && len(bold) == 0:
		return nil, ErrNoMarker
	case n == 1 && cueRaw == "":
		return nil, ErrEmptyCue
	case n == 1 && answerRaw == "":
		return nil, ErrEmptyAnswer
	}

	// raw is the fact without "::" but with bold markers; sentence is the
	// clean text shown on the back and used for the sentence hash, so adding,
	// moving or removing markers never changes the sentence's identity.
	raw := cueRaw
	if n == 1 {
		raw = cueRaw + " " + answerRaw
	}
	sentence := parser.StripBold(raw)
	hash := SentenceHash(sentence)

	var cards []Card
	seen := make(map[string]bool)
	add := func(c Card) {
		if seen[c.ClozeKey] {
			return
		}
		seen[c.ClozeKey] = true
		c.SentenceHash = hash
		c.Source = "template"
		cards = append(cards, c)
	}

	if n == 1 {
		cue, answer := parser.StripBold(cueRaw), parser.StripBold(answerRaw)
		c := Card{ClozeKey: ClozeKey(answer)}
		switch {
		case parser.IsQuestion(cue):
			c.Kind = KindQuestion
			c.Front = cue
			c.Back = answer
		case isDefinitionCue(topic, cue):
			verb := strings.ToLower(cue[len(topic)+1:])
			c.Kind = KindDefinition
			c.Front = fmt.Sprintf("What %s %s?", verb, cue[:len(topic)])
			c.Back = sentence
		default:
			c.Kind = KindCloze
			c.Front = cue + " ___" + trailPunctRe.FindString(answer)
			c.Back = sentence
		}
		add(c)
	}

	for i, sp := range parser.BoldSpans(raw) {
		add(Card{
			Kind:     KindCloze,
			ClozeKey: ClozeKey(sp.Text),
			Front:    parser.ReplaceBold(raw, i, "___"),
			Back:     sentence,
		})
	}
	return cards, nil
}

// isDefinitionCue reports whether the cue is exactly "TOPIC is" or "TOPIC are".
func isDefinitionCue(topic, cue string) bool {
	if topic == "" || len(cue) <= len(topic) {
		return false
	}
	if !strings.EqualFold(cue[:len(topic)], topic) {
		return false
	}
	rest := strings.ToLower(cue[len(topic):])
	return rest == " is" || rest == " are"
}

// GenerateCards generates all cards for a note deterministically from the
// markers in its facts (see FactCards). Facts without markers produce no card.
func GenerateCards(note *parser.Note) []Card {
	var cards []Card
	for _, fact := range note.Facts {
		fc, err := FactCards(note.Title, fact)
		if err != nil {
			continue
		}
		for _, c := range fc {
			c.NoteID = note.ID
			cards = append(cards, c)
		}
	}
	return cards
}
