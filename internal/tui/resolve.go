package tui

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/clobrano/mneme/internal/store"
)

var (
	panelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	highlightAdd  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	highlightDel  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	statusStyle   = lipgloss.NewStyle().Faint(true)
)

type resolvePhase int

const (
	phaseMain resolvePhase = iota
	phaseConfirmDiscard
)

// ResolveModel is the Bubbletea model for the orphan resolver.
type ResolveModel struct {
	db         *sql.DB
	orphans    []*store.DBCard
	orphanIdx  int
	candidates []*store.DBCard
	selected   int
	phase      resolvePhase
	batchID    string
	input      string
	status     string
	done       bool

	reviewRepo *store.ReviewRepository
	cardRepo   *store.CardRepository
}

// NewResolveModel creates a new ResolveModel.
func NewResolveModel(db *sql.DB, batchID string) (ResolveModel, error) {
	cardRepo := store.NewCardRepository(db)
	orphans, err := cardRepo.ListOrphaned()
	if err != nil {
		return ResolveModel{}, err
	}

	m := ResolveModel{
		db:         db,
		orphans:    orphans,
		batchID:    batchID,
		reviewRepo: store.NewReviewRepository(db),
		cardRepo:   cardRepo,
	}

	if len(orphans) > 0 {
		m.candidates, _ = cardRepo.ListActiveByNoteID(orphans[0].NoteID)
	}
	return m, nil
}

func (m ResolveModel) Init() tea.Cmd { return nil }

func (m ResolveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		if key == "ctrl+c" || key == "q" {
			return m, tea.Quit
		}

		if m.done {
			return m, tea.Quit
		}

		orphan := m.orphans[m.orphanIdx]

		switch m.phase {
		case phaseConfirmDiscard:
			if key == "y" {
				store.Discard(m.db, orphan.ID, m.batchID)
				m.status = "Discarded."
				m.advanceOrphan()
			} else {
				m.phase = phaseMain
				m.status = "Discard cancelled."
			}
			return m, nil

		case phaseMain:
			switch key {
			case "enter":
				// Enter does nothing — safety requirement
				return m, nil

			case "s":
				m.status = "Skipped."
				m.advanceOrphan()

			case "d":
				m.phase = phaseConfirmDiscard
				m.status = "Press y to confirm discard, any other key to cancel."

			case "up", "k":
				if m.selected > 0 {
					m.selected--
				}

			case "down", "j":
				if m.selected < len(m.candidates)-1 {
					m.selected++
				}

			case "w":
				// Widen to recent cards
				rows, err := m.db.Query(`
					SELECT id, note_id, sentence_hash, cloze_key, kind, front, back, status, source, created_at
					FROM cards WHERE status = 'active'
					ORDER BY created_at DESC LIMIT 20
				`)
				if err == nil {
					m.candidates = scanCardRows(rows)
					rows.Close()
				}
				m.selected = 0
				m.status = "Showing recent active cards."

			default:
				// Check for r<n>y pattern from input buffer
				m.input += key
				if len(m.input) >= 3 && strings.HasPrefix(m.input, "r") {
					rest := m.input[1:]
					if strings.HasSuffix(rest, "y") {
						numStr := rest[:len(rest)-1]
						idx, err := strconv.Atoi(numStr)
						if err == nil && idx >= 1 && idx <= len(m.candidates) {
							target := m.candidates[idx-1]
							if err := store.Remap(m.db, orphan.ID, target.ID, m.batchID); err != nil {
								m.status = fmt.Sprintf("Remap error: %v", err)
							} else {
								m.status = fmt.Sprintf("Remapped to card [%d].", target.ID)
								m.advanceOrphan()
							}
							m.input = ""
						} else {
							m.input = ""
						}
					}
				} else if len(m.input) > 5 {
					m.input = ""
				}
			}
		}
	}
	return m, nil
}

func (m *ResolveModel) advanceOrphan() {
	m.orphanIdx++
	m.selected = 0
	m.phase = phaseMain
	m.input = ""
	if m.orphanIdx >= len(m.orphans) {
		m.done = true
		return
	}
	orphan := m.orphans[m.orphanIdx]
	m.candidates, _ = m.cardRepo.ListActiveByNoteID(orphan.NoteID)
}

func (m ResolveModel) View() string {
	if m.done {
		return "\nAll orphans resolved.\n\nPress q to exit.\n"
	}

	orphan := m.orphans[m.orphanIdx]
	rev, _ := m.reviewRepo.GetByCardID(orphan.ID)

	// Left panel: orphan info
	var left strings.Builder
	left.WriteString(boldStyle.Render(fmt.Sprintf("Orphan [%d/%d]", m.orphanIdx+1, len(m.orphans))))
	left.WriteString("\n\n")
	left.WriteString(boldStyle.Render("Sentence:"))
	left.WriteString("\n")
	left.WriteString(orphan.Back)
	left.WriteString("\n\n")
	if rev != nil {
		left.WriteString(dimStyle.Render(fmt.Sprintf("reps=%d  lapses=%d  due=%s",
			rev.Reps, rev.Lapses, rev.Due.Format("2006-01-02"))))
	}

	// Right panel: candidates
	var right strings.Builder
	right.WriteString(boldStyle.Render("Candidates:"))
	right.WriteString("\n\n")
	if len(m.candidates) == 0 {
		right.WriteString(dimStyle.Render("(none — use w to widen or /keyword to search)"))
	} else {
		for i, c := range m.candidates {
			line := fmt.Sprintf("[%d] (%s) %s", i+1, c.Kind, truncate(c.Back, 50))
			if i == m.selected {
				right.WriteString(selectedStyle.Render(line))
			} else {
				right.WriteString(line)
			}
			if i < len(m.candidates)-1 {
				right.WriteString("\n")
			}
			// Show diff for selected candidate
			if i == m.selected {
				diff := wordDiff(orphan.Back, c.Back)
				right.WriteString("\n  ")
				right.WriteString(dimStyle.Render("diff: "))
				right.WriteString(diff)
			}
			right.WriteString("\n")
		}
	}

	leftPanel := panelStyle.Width(40).Render(left.String())
	rightPanel := panelStyle.Width(50).Render(right.String())
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	// Bottom bar
	bottom := statusStyle.Render("r<n>y=remap  d(y)=discard  s=skip  w=widen  ↑↓=select  q=quit")
	if m.status != "" {
		bottom = m.status + "  │  " + bottom
	}

	return "\n" + panels + "\n\n" + bottom + "\n"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// wordDiff produces a simple word-level diff highlighting added/removed words.
func wordDiff(a, b string) string {
	wa := strings.Fields(a)
	wb := strings.Fields(b)

	aSet := make(map[string]bool)
	bSet := make(map[string]bool)
	for _, w := range wa {
		aSet[w] = true
	}
	for _, w := range wb {
		bSet[w] = true
	}

	var parts []string
	for _, w := range wb {
		if !aSet[w] {
			parts = append(parts, highlightAdd.Render("+"+w))
		} else {
			parts = append(parts, w)
		}
	}
	for _, w := range wa {
		if !bSet[w] {
			parts = append(parts, highlightDel.Render("-"+w))
		}
	}
	return strings.Join(parts, " ")
}

func scanCardRows(rows *sql.Rows) []*store.DBCard {
	var cards []*store.DBCard
	for rows.Next() {
		c := &store.DBCard{}
		var createdAt string
		rows.Scan(&c.ID, &c.NoteID, &c.SentenceHash, &c.ClozeKey, &c.Kind, &c.Front, &c.Back, &c.Status, &c.Source, &createdAt)
		cards = append(cards, c)
	}
	return cards
}

// RunResolveTUI launches the Bubbletea resolver TUI.
func RunResolveTUI(db *sql.DB) error {
	batchID := fmt.Sprintf("%d", time.Now().UnixNano())
	model, err := NewResolveModel(db, batchID)
	if err != nil {
		return err
	}
	if len(model.orphans) == 0 {
		fmt.Println("No orphaned cards to resolve.")
		return nil
	}
	_, err = tea.NewProgram(model).Run()
	return err
}
