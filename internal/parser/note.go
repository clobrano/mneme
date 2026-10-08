package parser

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Note represents a parsed atomic note.
type Note struct {
	Title    string
	Tags     []string
	Facts    []string
	FilePath string
	// ID is set by the store layer after the note is persisted.
	ID int64
}

var (
	titleRe   = regexp.MustCompile(`^#\s+(.+)$`)
	tagLineRe = regexp.MustCompile(`^(#[\w][\w/-]*(\s+#[\w][\w/-]*)*)\s*$`)
	wikiRe    = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
)

// ParseNote reads a note file and extracts title, tags, and fact sentences.
// Returns an error only for unreadable files, never for format issues.
func ParseNote(path string) (*Note, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading note %s: %w", path, err)
	}

	note := &Note{FilePath: path}
	lines := strings.Split(string(data), "\n")

	// Phase 1: Find title
	titleIdx := -1
	for i, line := range lines {
		if m := titleRe.FindStringSubmatch(line); m != nil {
			note.Title = strings.TrimSpace(m[1])
			titleIdx = i
			break
		}
	}

	if titleIdx == -1 {
		// No title found; treat all non-empty content as facts
		content := strings.TrimSpace(string(data))
		if content != "" {
			blocks := splitBlocks(content)
			note.Facts = blocks
		}
		return note, nil
	}

	// Phase 2: Extract tags from header block (lines after title, before first blank line)
	i := titleIdx + 1
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			// End of header block
			i++
			break
		}
		if m := tagLineRe.FindStringSubmatch(line); m != nil {
			// Split the line on whitespace to get individual tags
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.HasPrefix(p, "#") {
					tag := strings.ToLower(strings.TrimPrefix(p, "#"))
					note.Tags = append(note.Tags, tag)
				}
			}
		}
		i++
	}

	// Phase 3: Extract fact sentences from remaining content
	remaining := strings.Join(lines[i:], "\n")
	remaining = strings.TrimSpace(remaining)
	if remaining != "" {
		note.Facts = splitBlocks(remaining)
	}

	// Phase 4: Warn about facts not starting with TOPIC
	if note.Title != "" {
		titleLower := strings.ToLower(note.Title)
		for _, fact := range note.Facts {
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(fact)), titleLower) {
				fmt.Fprintf(os.Stderr, "warning: fact in %s does not begin with topic %q: %s\n",
					path, note.Title, fact)
			}
		}
	}

	return note, nil
}

// splitBlocks splits text on one or more blank lines, returning non-empty trimmed blocks.
func splitBlocks(text string) []string {
	// Split on two or more consecutive newlines (blank line separator)
	blankLineRe := regexp.MustCompile(`\n\n+`)
	parts := blankLineRe.Split(text, -1)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ExtractWikiLinks returns the inner texts of all [[...]] occurrences in order.
func ExtractWikiLinks(sentence string) []string {
	matches := wikiRe.FindAllStringSubmatch(sentence, -1)
	var result []string
	for _, m := range matches {
		result = append(result, m[1])
	}
	return result
}
