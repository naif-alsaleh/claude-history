package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/naif/claude-history/internal/data"
	"github.com/naif/claude-history/internal/search"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	researchStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	dateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	urlStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("69")).
			Underline(true)

	snippetStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	selectedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("62")).
			Padding(0, 1).
			Width(80)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

type model struct {
	input      textinput.Model
	viewport   viewport.Model
	searcher   search.Searcher
	results    []data.SearchResult
	cursor     int
	width      int
	height     int
	searching  bool
	lastQuery  string
}

type searchDoneMsg struct {
	results []data.SearchResult
	err     error
}

func Run(searcher search.Searcher) error {
	ti := textinput.New()
	ti.Placeholder = "Search conversations..."
	ti.Focus()
	ti.Width = 60

	vp := viewport.New(80, 20)

	m := model{
		input:    ti,
		viewport: vp,
		searcher: searcher,
		width:    80,
		height:   24,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if len(m.results) > 0 {
				openURL(fmt.Sprintf("https://claude.ai/chat/%s", m.results[m.cursor].Conversation.UUID))
			}
			return m, nil
		case "up", "ctrl+k":
			if m.cursor > 0 {
				m.cursor--
				m.viewport.SetContent(m.renderResults())
			}
			return m, nil
		case "down", "ctrl+j":
			if m.cursor < len(m.results)-1 {
				m.cursor++
				m.viewport.SetContent(m.renderResults())
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerH := 4 // header + input + help
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerH
		m.input.Width = msg.Width - 4
		if len(m.results) > 0 {
			m.viewport.SetContent(m.renderResults())
		}

	case searchDoneMsg:
		m.searching = false
		if msg.err != nil {
			m.viewport.SetContent(fmt.Sprintf("Error: %v", msg.err))
		} else {
			m.results = msg.results
			m.cursor = 0
			m.viewport.SetContent(m.renderResults())
			m.viewport.GotoTop()
		}
		return m, nil
	}

	prevVal := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	if m.input.Value() != prevVal {
		query := m.input.Value()
		if query != m.lastQuery && len(query) >= 2 {
			m.lastQuery = query
			m.searching = true
			cmds = append(cmds, m.doSearch(query))
		} else if len(query) < 2 {
			m.results = nil
			m.viewport.SetContent("")
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) doSearch(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := m.searcher.Search(context.Background(), query, 50)
		return searchDoneMsg{results: results, err: err}
	}
}

func (m model) View() string {
	header := headerStyle.Width(m.width).Render("Claude History Search")

	status := ""
	if m.searching {
		status = " searching..."
	} else if len(m.results) > 0 {
		status = fmt.Sprintf(" %d results", len(m.results))
	}

	help := helpStyle.Render("↑/↓ navigate • enter open in browser • esc quit" + status)

	return fmt.Sprintf("%s\n%s\n%s\n%s",
		header,
		m.input.View(),
		help,
		m.viewport.View(),
	)
}

func (m model) renderResults() string {
	if len(m.results) == 0 {
		return "No results found."
	}

	var b strings.Builder
	for i, r := range m.results {
		entry := m.renderEntry(i, r)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(entry))
		} else {
			b.WriteString(normalStyle.Render(entry))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) renderEntry(idx int, r data.SearchResult) string {
	var b strings.Builder

	title := titleStyle.Render(fmt.Sprintf("%d. %s", idx+1, r.Conversation.Name))
	if r.Conversation.IsResearch {
		title += " " + researchStyle.Render("Research")
	}
	b.WriteString(title)
	b.WriteString("\n")

	date := dateStyle.Render(r.Conversation.CreatedAt.Format(time.DateOnly))
	url := urlStyle.Render(fmt.Sprintf("https://claude.ai/chat/%s", r.Conversation.UUID))
	b.WriteString(fmt.Sprintf("%s  %s", date, url))
	b.WriteString("\n")

	if r.Snippet != "" {
		snippet := snippetStyle.Render(truncate(r.Snippet, m.width-6))
		b.WriteString(snippet)
	}

	return b.String()
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 80
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return
	}
	cmd.Start()
}
