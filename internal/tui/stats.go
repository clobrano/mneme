package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/clobrano/mneme/internal/store"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	countStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	barStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
)

// StatsModel is the Bubbletea model for the stats screen.
type StatsModel struct {
	active   int
	dueToday int
	orphaned int
	forecast []store.ForecastEntry
	loaded   bool
}

type statsLoaded struct {
	active   int
	dueToday int
	orphaned int
	forecast []store.ForecastEntry
}

func loadStats(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
		active, _ := store.CountActive(db)
		dueToday, _ := store.CountDueToday(db, endOfDay)
		orphaned, _ := store.CountOrphaned(db)
		forecast, _ := store.DueForecast(db, now)
		return statsLoaded{active, dueToday, orphaned, forecast}
	}
}

// NewStatsModel creates an unloaded StatsModel.
func NewStatsModel() StatsModel { return StatsModel{} }

func (m StatsModel) Init() tea.Cmd { return nil }

func (m StatsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statsLoaded:
		m.active = msg.active
		m.dueToday = msg.dueToday
		m.orphaned = msg.orphaned
		m.forecast = msg.forecast
		m.loaded = true
	case tea.KeyMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m StatsModel) View() string {
	if !m.loaded {
		return "\nLoading stats...\n"
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render("Deck Statistics"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Active cards:  %s\n", countStyle.Render(fmt.Sprintf("%d", m.active))))
	sb.WriteString(fmt.Sprintf("Due today:     %s\n", countStyle.Render(fmt.Sprintf("%d", m.dueToday))))
	sb.WriteString(fmt.Sprintf("Orphaned:      %s\n", countStyle.Render(fmt.Sprintf("%d", m.orphaned))))
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render("7-day Forecast"))
	sb.WriteString("\n\n")

	if len(m.forecast) == 0 {
		sb.WriteString(dimStyle.Render("(no cards due in the next 7 days)"))
		sb.WriteString("\n")
	} else {
		maxCount := 1
		for _, e := range m.forecast {
			if e.Count > maxCount {
				maxCount = e.Count
			}
		}
		for _, e := range m.forecast {
			barLen := e.Count * 30 / maxCount
			bar := strings.Repeat("█", barLen)
			sb.WriteString(fmt.Sprintf("  %-12s %3d %s\n",
				e.Date, e.Count, barStyle.Render(bar)))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("Press any key to exit."))
	sb.WriteString("\n")
	return sb.String()
}

// RunStatsTUI launches the Bubbletea stats TUI.
func RunStatsTUI(db *sql.DB) error {
	model := NewStatsModel()
	p := tea.NewProgram(model)
	go func() {
		msg := loadStats(db)()
		p.Send(msg)
	}()
	_, err := p.Run()
	return err
}
