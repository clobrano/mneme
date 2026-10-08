package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/clobrano/mneme/internal/scheduler"
	"github.com/clobrano/mneme/internal/store"
)

var (
	boldStyle      = lipgloss.NewStyle().Bold(true)
	dimStyle       = lipgloss.NewStyle().Faint(true)
	gradeAgain     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	gradeHard      = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	gradeGood      = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	gradeEasy      = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	progressStyle  = lipgloss.NewStyle().Faint(true)
)

type reviewState int

const (
	stateShowFront reviewState = iota
	stateShowBack
	stateDone
)

// ReviewModel is the Bubbletea model for the review session.
type ReviewModel struct {
	db         *sql.DB
	cards      []*store.DBCard
	index      int
	state      reviewState
	reviewRepo *store.ReviewRepository
	logRepo    *store.ReviewLogRepository
}

// NewReviewModel creates a new ReviewModel loaded with due cards.
func NewReviewModel(db *sql.DB, cards []*store.DBCard) ReviewModel {
	return ReviewModel{
		db:         db,
		cards:      cards,
		index:      0,
		state:      stateShowFront,
		reviewRepo: store.NewReviewRepository(db),
		logRepo:    store.NewReviewLogRepository(db),
	}
}

func (m ReviewModel) Init() tea.Cmd { return nil }

func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateShowFront:
			// Any key reveals the back
			m.state = stateShowBack

		case stateShowBack:
			var grade int
			switch msg.String() {
			case "1":
				grade = 1
			case "2":
				grade = 2
			case "3":
				grade = 3
			case "4":
				grade = 4
			case "q", "ctrl+c":
				return m, tea.Quit
			default:
				return m, nil
			}
			m.persistGrade(grade)
			m.index++
			if m.index >= len(m.cards) {
				m.state = stateDone
			} else {
				m.state = stateShowFront
			}

		case stateDone:
			return m, tea.Quit
		}

		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ReviewModel) View() string {
	switch m.state {
	case stateDone:
		return fmt.Sprintf("\n%s\n\nPress any key to exit.\n",
			boldStyle.Render(fmt.Sprintf("Session complete! Reviewed %d card(s).", len(m.cards))))

	case stateShowFront, stateShowBack:
		card := m.cards[m.index]
		progress := progressStyle.Render(fmt.Sprintf("Card %d of %d", m.index+1, len(m.cards)))
		kind := dimStyle.Render(fmt.Sprintf("[%s]", strings.ToUpper(card.Kind)))

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("\n%s  %s\n\n", progress, kind))
		sb.WriteString(boldStyle.Render(card.Front))
		sb.WriteString("\n")

		if m.state == stateShowBack {
			sb.WriteString("\n")
			sb.WriteString(dimStyle.Render("Answer:"))
			sb.WriteString("\n")
			sb.WriteString(card.Back)
			sb.WriteString("\n\n")
			sb.WriteString(gradeAgain.Render("1=again") + "  ")
			sb.WriteString(gradeHard.Render("2=hard") + "  ")
			sb.WriteString(gradeGood.Render("3=good") + "  ")
			sb.WriteString(gradeEasy.Render("4=easy"))
			sb.WriteString("\n")
		} else {
			sb.WriteString("\n")
			sb.WriteString(dimStyle.Render("Press any key to reveal..."))
			sb.WriteString("\n")
		}
		return sb.String()
	}
	return ""
}

func (m *ReviewModel) persistGrade(grade int) {
	card := m.cards[m.index]
	rev, err := m.reviewRepo.GetByCardID(card.ID)
	if err != nil || rev == nil {
		return
	}
	updated := scheduler.Grade(rev, grade, time.Now())
	m.reviewRepo.Update(updated)
	m.logRepo.Insert(card.ID, grade)
}

// RunReviewTUI launches the Bubbletea review TUI.
func RunReviewTUI(db *sql.DB, cards []*store.DBCard) error {
	if len(cards) == 0 {
		fmt.Println("No cards due. Great job!")
		return nil
	}
	model := NewReviewModel(db, cards)
	_, err := tea.NewProgram(model).Run()
	return err
}
